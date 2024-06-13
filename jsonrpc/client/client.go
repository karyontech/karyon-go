package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
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
)

// RPCClientConfig Holds the configuration settings for the RPC client.
type RPCClientConfig struct {
	Timeout int    // Timeout for receiving requests from the server, in milliseconds.
	Addr    string // Address of the RPC server.
}

// RPCClient RPC Client
type RPCClient struct {
	config      RPCClientConfig
	conn        *websocket.Conn
	requests    messageDispatcher[message.RequestID, message.Response]
	subscriber  messageDispatcher[message.SubscriptionID, json.RawMessage]
	stop_signal chan struct{}
}

// NewRPCClient Creates a new instance of RPCClient with the provided configuration.
// It establishes a WebSocket connection to the RPC server.
func NewRPCClient(config RPCClientConfig) (*RPCClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(config.Addr, nil)
	if err != nil {
		return nil, err
	}
	log.Infof("Successfully connected to the server: %s", config.Addr)

	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}

	stop_signal := make(chan struct{}, 2)

	client := &RPCClient{
		conn:        conn,
		config:      config,
		requests:    newMessageDispatcher[message.RequestID, message.Response](1),
		subscriber:  newMessageDispatcher[message.SubscriptionID, json.RawMessage](10),
		stop_signal: stop_signal,
	}

	go func() {
		if err := client.backgroundReceivingLoop(stop_signal); err != nil {
			client.Close()
		}
	}()

	return client, nil
}

// Close Closes the underlying websocket connection and stop the receiving loop.
func (client *RPCClient) Close() {
	log.Warn("Close the rpc client...")
	// Send stop signal to the background receiving loop
	client.stop_signal <- struct{}{}

	// Close the underlying websocket connection
	err := client.conn.Close()
	if err != nil {
		log.WithError(err).Error("Close websocket connection")
	}

	client.requests.clear()
	client.subscriber.clear()
}

// Call Sends an RPC call to the server with the specified method and
// parameters, and returns the response.
func (client *RPCClient) Call(method string, params any) (*json.RawMessage, error) {
	log.Tracef("Call -> method: %s, params: %v", method, params)
	response, err := client.sendRequest(method, params)
	if err != nil {
		return nil, err
	}

	return response.Result, nil
}

// Subscribe Sends a subscription request to the server with the specified
// method and parameters, and it returns the subscription ID and the channel to
// receive notifications.
func (client *RPCClient) Subscribe(method string, params any) (message.SubscriptionID, <-chan json.RawMessage, error) {
	log.Tracef("Sbuscribe ->  method: %s, params: %v", method, params)
	response, err := client.sendRequest(method, params)
	if err != nil {
		return 0, nil, err
	}

	if response.Result == nil {
		return 0, nil, fmt.Errorf("Invalid response result")
	}

	var subID message.SubscriptionID
	err = json.Unmarshal(*response.Result, &subID)
	if err != nil {
		return 0, nil, err
	}

	// Register a new subscription
	sub := client.subscriber.register(subID)

	return subID, sub, nil
}

// Unsubscribe Sends an unsubscription request to the server to cancel the
// given subscription.
func (client *RPCClient) Unsubscribe(method string, subID message.SubscriptionID) error {
	log.Tracef("Unsubscribe -> method: %s, subID: %d", method, subID)
	_, err := client.sendRequest(method, subID)
	if err != nil {
		return err
	}

	// On success unregister the subscription channel
	client.subscriber.unregister(subID)

	return nil
}

// backgroundReceivingLoop Starts reading new messages from the underlying connection.
func (client *RPCClient) backgroundReceivingLoop(stop_signal <-chan struct{}) error {

	log.Debug("Background loop started")

	new_msg_ch := make(chan []byte)
	receive_err_ch := make(chan error)

	// Start listing for new messages
	go func() {
		for {
			_, msg, err := client.conn.ReadMessage()
			if err != nil {
				receive_err_ch <- err
				return
			}
			new_msg_ch <- msg
		}
	}()

	for {
		select {
		case <-stop_signal:
			log.Warn("Stopping background receiving loop: received stop signal")
			return nil
		case msg := <-new_msg_ch:
			err := client.handleNewMsg(msg)
			if err != nil {
				log.WithError(err).Error("Handle a new received msg")
			}
		case err := <-receive_err_ch:
			log.WithError(err).Error("Receive a new msg")
			return err
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
			return fmt.Errorf("Response doesn't have an id")
		}

		err := client.requests.disptach(*response.ID, response)
		if err != nil {
			return fmt.Errorf("Dispatch a response: %w", err)
		}

		return nil
	}

	// Check if the received message is of type Notification
	notification := message.Notification{}
	if err := json.Unmarshal(msg, &notification); err == nil {

		ntRes := message.NotificationResult{}
		if err := json.Unmarshal(*notification.Params, &ntRes); err != nil {
			return fmt.Errorf("Failed to unmarshal notification params: %w", err)
		}

		// Send the notification to the subscription
		err := client.subscriber.disptach(ntRes.Subscription, *ntRes.Result)
		if err != nil {
			return fmt.Errorf("Dispatch a notification: %w", err)
		}

		log.Debugf("<-- %s", notification.String())

		return nil
	}

	return fmt.Errorf("Receive unexpected msg: %s", msg)
}

// sendRequest Sends a request and wait for the response
func (client *RPCClient) sendRequest(method string, params any) (message.Response, error) {
	response := message.Response{}

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
	case <-time.After(time.Duration(client.config.Timeout) * time.Millisecond):
		return response, fmt.Errorf("Timeout error")
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
			return fmt.Errorf("Invalid response id")
		}
	}

	return nil
}
