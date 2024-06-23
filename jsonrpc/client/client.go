package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"github.com/karyontech/karyon-go/jsonrpc/message"
)

const (
	// JsonRPCVersion Defines the version of the JSON-RPC protocol being used.
	JsonRPCVersion = "2.0"

	// Default timeout for receiving requests from the server, in milliseconds.
	DefaultTimeout = 3000

	// The default buffer size for a subscription.
	DefaultSubscriptionBufferSize = 10000
)

var (
	ClientIsDisconnectedErr  = errors.New("Client is disconnected and closed")
	TimeoutError             = errors.New("Timeout Error")
	InvalidResponseIDErr     = errors.New("Invalid response ID")
	InvalidResponseResultErr = errors.New("Invalid response result")
	receivedStopSignalErr    = errors.New("Received stop signal")
)

// RPCClientConfig Holds the configuration settings for the RPC client.
type RPCClientConfig struct {
	Timeout                int    // Timeout for receiving requests from the server, in milliseconds.
	Addr                   string // Address of the RPC server.
	SubscriptionBufferSize int    // The buffer size for a subscription.
}

// RPCClient RPC Client
type RPCClient struct {
	config        RPCClientConfig
	conn          *websocket.Conn
	requests      *messageDispatcher
	subscriptions *subscriptions
	stopSignal    chan struct{}
	isClosed      atomic.Bool
}

// NewRPCClient Creates a new instance of RPCClient with the provided configuration.
// It establishes a WebSocket connection to the RPC server.
func NewRPCClient(config RPCClientConfig) (*RPCClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(config.Addr, nil)
	if err != nil {
		return nil, err
	}
	log.Infof("Successfully connected to the server: %s", config.Addr)

	if config.Timeout <= 0 {
		config.Timeout = DefaultTimeout
	}

	if config.SubscriptionBufferSize <= 0 {
		config.SubscriptionBufferSize = DefaultSubscriptionBufferSize
	}

	stopSignal := make(chan struct{})

	requests := newMessageDispatcher()
	subscriptions := newSubscriptions(config.SubscriptionBufferSize)

	client := &RPCClient{
		conn:          conn,
		config:        config,
		requests:      requests,
		subscriptions: subscriptions,
		stopSignal:    stopSignal,
	}

	go func() {
		if err := client.backgroundReceivingLoop(stopSignal); err != nil {
			client.Close()
		}
	}()

	return client, nil
}

// Close Closes the underlying websocket connection and stop the receiving loop.
func (client *RPCClient) Close() {
	// Check if it's already closed
	if !client.isClosed.CompareAndSwap(false, true) {
		return
	}

	log.Warn("Close the rpc client...")
	// Send stop signal to the background receiving loop
	close(client.stopSignal)

	// Close the underlying websocket connection
	err := client.conn.Close()
	if err != nil {
		log.WithError(err).Error("Close websocket connection")
	}

	client.requests.clear()
	client.subscriptions.clear()
}

// Call Sends an RPC call to the server with the specified method and
// parameters, and returns the response.
func (client *RPCClient) Call(method string, params any) (json.RawMessage, error) {
	log.Tracef("Call -> method: %s, params: %v", method, params)
	response, err := client.sendRequest(method, params)
	if err != nil {
		return nil, err
	}

	return response.Result, nil
}

// Subscribe Sends a subscription request to the server with the specified
// method and parameters, and it returns the subscription.
func (client *RPCClient) Subscribe(method string, params any) (*Subscription, error) {
	log.Tracef("Sbuscribe ->  method: %s, params: %v", method, params)
	response, err := client.sendRequest(method, params)
	if err != nil {
		return nil, err
	}

	if response.Result == nil {
		return nil, InvalidResponseResultErr
	}

	var subID message.SubscriptionID
	err = json.Unmarshal(response.Result, &subID)
	if err != nil {
		return nil, err
	}

	sub := client.subscriptions.subscribe(subID)

	return sub, nil
}

// Unsubscribe Sends an unsubscription request to the server to cancel the
// given subscription.
func (client *RPCClient) Unsubscribe(method string, subID message.SubscriptionID) error {
	log.Tracef("Unsubscribe -> method: %s, subID: %d", method, subID)
	_, err := client.sendRequest(method, subID)
	if err != nil {
		return err
	}

	// On success unsubscribe
	client.subscriptions.unsubscribe(subID)

	return nil
}

// backgroundReceivingLoop Starts reading new messages from the underlying connection.
func (client *RPCClient) backgroundReceivingLoop(stopSignal <-chan struct{}) error {
	log.Debug("Background loop started")

	newMsgCh := make(chan []byte)
	receiveErrCh := make(chan error)

	// Start listing for new messages
	go func() {
		for {
			_, msg, err := client.conn.ReadMessage()
			if err != nil {
				receiveErrCh <- err
				return
			}
			select {
			case <-client.stopSignal:
				return
			case newMsgCh <- msg:
			}
		}
	}()

	for {
		select {
		case err := <-receiveErrCh:
			log.WithError(err).Error("Read a new msg")
			return err
		case <-stopSignal:
			log.Debug("Background receiving loop stopped %w", receivedStopSignalErr)
			return nil
		case msg := <-newMsgCh:
			err := client.handleNewMsg(msg)
			if err != nil {
				log.WithError(err).Error("Handle a msg")
				return err
			}
		}
	}
}

// handleNewMsg Attempts to decode the received message into either a Response
// or Notification.
func (client *RPCClient) handleNewMsg(msg []byte) error {
	// Check if the received message is of type Response
	response := message.Response{}
	decoder := json.NewDecoder(bytes.NewReader(msg))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err == nil {
		if response.ID == nil {
			return InvalidResponseIDErr
		}

		err := client.requests.dispatch(*response.ID, response)
		if err != nil {
			return fmt.Errorf("Dispatch a response: %w", err)
		}

		return nil
	}

	// Check if the received message is of type Notification
	notification := message.Notification{}
	if err := json.Unmarshal(msg, &notification); err == nil {

		ntRes := message.NotificationResult{}
		if err := json.Unmarshal(notification.Params, &ntRes); err != nil {
			return fmt.Errorf("Failed to unmarshal notification params: %w", err)
		}

		err := client.subscriptions.notify(ntRes.Subscription, ntRes.Result)
		if err != nil {
			return fmt.Errorf("Notify a subscriber: %w", err)
		}

		log.Debugf("<-- %s", notification.String())

		return nil
	}

	return fmt.Errorf("Receive unexpected msg: %s", msg)
}

// sendRequest Sends a request and wait for the response
func (client *RPCClient) sendRequest(method string, params any) (message.Response, error) {
	response := message.Response{}

	if client.isClosed.Load() {
		return response, ClientIsDisconnectedErr
	}

	params_bytes, err := json.Marshal(params)
	if err != nil {
		return response, err
	}

	params_raw := json.RawMessage(params_bytes)

	// Generate a new id
	id := strconv.Itoa(rand.Int())
	req := message.Request{
		JSONRPC: JsonRPCVersion,
		ID:      id,
		Method:  method,
		Params:  &params_raw,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return response, err
	}

	err = client.conn.WriteMessage(websocket.TextMessage, []byte(string(reqJSON)))
	if err != nil {
		return response, err
	}

	log.Debugf("--> %s", req.String())

	rx_ch := client.requests.register(id)
	defer client.requests.unregister(id)

	// Waits the response, it fails and return error if it exceed the timeout
	select {
	case response = <-rx_ch:
	case <-client.stopSignal:
		return response, ClientIsDisconnectedErr
	case <-time.After(time.Duration(client.config.Timeout) * time.Millisecond):
		return response, TimeoutError
	}

	err = validateResponse(&response, id)
	if err != nil {
		return response, err
	}

	log.Debugf("<-- %s", response.String())

	return response, nil
}

// validateResponse Checks the error field and whether the request id is the
// same as the response id
func validateResponse(res *message.Response, reqID message.RequestID) error {
	if res.Error != nil {
		return fmt.Errorf("Receive An Error: %s", res.Error.String())
	}

	if res.ID != nil {
		if *res.ID != reqID {
			return InvalidResponseIDErr
		}
	}

	return nil
}
