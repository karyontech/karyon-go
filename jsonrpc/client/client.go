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
	"github.com/karyontech/karyon-go/jsonrpc/util"
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
	requests      *util.MessageDispatcher
	subscriptions *util.Subscriptions
	stopSignal    chan struct{}
	isClosed      atomic.Bool
}

// NewRPCClient creates a new instance of RPCClient with the provided configuration.
// It establishes a WebSocket connection to the RPC server and starts a background receiving loop.
// Returns an error if the connection cannot be established.
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

	requests := util.NewMessageDispatcher()
	subscriptions := util.NewSubscriptions(config.SubscriptionBufferSize)

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

// Close gracefully shuts down the RPC client by closing the WebSocket connection,
// stopping the background receiving loop, and cleaning up all resources.
// It ensures the client can only be closed once using atomic operations.
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

	client.requests.Close()
	client.subscriptions.Close()
}

// Call sends a synchronous RPC call to the server with the specified method and parameters.
// It waits for and returns the response result, or an error if the call fails.
func (client *RPCClient) Call(method string, params any) (json.RawMessage, error) {
	log.Tracef("Call -> method: %s, params: %v", method, params)
	response, err := client.sendRequest(method, params)
	if err != nil {
		return nil, err
	}

	return response.Result, nil
}

// Subscribe sends a subscription request to the server and returns a Subscription object.
// The subscription can be used to receive notifications from the server for the specified method.
func (client *RPCClient) Subscribe(method string, params any) (*util.Subscription, error) {
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

	sub := client.subscriptions.Subscribe(subID)

	return sub, nil
}

// Unsubscribe sends an unsubscription request to the server to cancel the specified subscription.
// It removes the subscription from the local subscription manager upon successful completion.
func (client *RPCClient) Unsubscribe(method string, subID message.SubscriptionID) error {
	log.Tracef("Unsubscribe -> method: %s, subID: %d", method, subID)
	_, err := client.sendRequest(method, subID)
	if err != nil {
		return err
	}

	// On success unsubscribe
	client.subscriptions.Unsubscribe(subID)

	return nil
}

// backgroundReceivingLoop continuously reads messages from the WebSocket connection in a separate goroutine.
// It handles incoming responses and notifications, dispatching them to the appropriate handlers.
// The loop terminates when a stop signal is received or an error occurs.
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

// handleNewMsg processes incoming messages by attempting to decode them as either Response or Notification.
// For responses, it dispatches them to waiting request handlers. For notifications, it forwards them to subscribers.
// Returns an error if the message cannot be processed or is malformed.
func (client *RPCClient) handleNewMsg(msg []byte) error {
	// Check if the received message is of type Response
	response := message.Response{}
	decoder := json.NewDecoder(bytes.NewReader(msg))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err == nil {
		if response.ID == nil {
			return InvalidResponseIDErr
		}

		err := client.requests.Dispatch(*response.ID, response)
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

		err := client.subscriptions.Notify(ntRes.Subscription, ntRes.Result)
		if err != nil {
			return fmt.Errorf("Notify a subscriber: %w", err)
		}

		log.Debugf("<-- %s", notification.String())

		return nil
	}

	return fmt.Errorf("Receive unexpected msg: %s", msg)
}

// sendRequest sends a JSON-RPC request to the server and waits for the response.
// It generates a unique request ID, marshals the request, sends it over WebSocket, and waits for a response.
// Returns an error if the client is disconnected, marshaling fails, or a timeout occurs.
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

	rx_ch := client.requests.Register(id)
	defer client.requests.Unregister(id)

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

// validateResponse verifies that the response is valid by checking for errors and ensuring the response ID matches the request ID.
// It returns an error if the response contains an error field or if the response ID doesn't match the request ID.
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
