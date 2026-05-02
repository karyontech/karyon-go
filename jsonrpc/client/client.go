package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/karyontech/karyon-jsonrpc-go/jsonrpc/message"
)

const (
	// JsonRPCVersion Defines the version of the JSON-RPC protocol being used.
	JsonRPCVersion = "2.0"

	// Default timeout for receiving requests from the server, in milliseconds.
	DefaultTimeout = 3000

	// The default buffer size for a subscription.
	DefaultSubscriptionBufferSize = 10000

	// The default message buffer size for reading messages from the connection.
	DefaultMessageBufferSize = 512
)

var (
	ClientIsDisconnectedError  = errors.New("Client is disconnected and closed")
	TimeoutError               = errors.New("Timeout Error")
	InvalidResponseIDError     = errors.New("Invalid response ID")
	InvalidResponseResultError = errors.New("Invalid response result")
)

// RPCClientConfig Holds the configuration settings for the RPC client.
type RPCClientConfig struct {
	Timeout                int         // Timeout for receiving requests from the server, in milliseconds.
	Addr                   string      // Address of the RPC server.
	SubscriptionBufferSize int         // The buffer size for a subscription.
	MessageBufferSize      int         // The buffer size for reading messages from the connection.
	ChannelBufferSize      int         // The buffer size for response channels in MessageDispatcher.
	TLSConfig              *tls.Config // TLS configuration; required for tls:// addresses, ignored otherwise.
}

// RPCClient RPC Client
type RPCClient struct {
	config        RPCClientConfig
	conn          net.Conn
	requests      *messageDispatcher
	subscriptions *subscriptions
	stopSignal    chan struct{}
	isClosed      atomic.Bool
}

// NewRPCClient creates a new instance of RPCClient with the provided configuration.
// It establishes a WebSocket connection to the RPC server and starts a background receiving loop.
// Returns an error if the connection cannot be established.
func NewRPCClient(config RPCClientConfig) (*RPCClient, error) {
	u, err := url.Parse(config.Addr)
	if err != nil {
		return nil, fmt.Errorf("parsing the url: %w", err)
	}

	var conn net.Conn

	slog.Info("Connecting to the server...", "url", u)
	switch u.Scheme {
	case "tcp":
		conn, err = net.Dial("tcp", u.Host)
		if err != nil {
			return nil, err
		}
	case "tls":
		conn, err = tls.Dial("tcp", u.Host, config.TLSConfig)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Unsupported protocol: %s", u.Scheme)
	}

	slog.Info("Successfully connected to the server", "url", u)

	if config.Timeout <= 0 {
		config.Timeout = DefaultTimeout
	}

	if config.SubscriptionBufferSize <= 0 {
		config.SubscriptionBufferSize = DefaultSubscriptionBufferSize
	}

	if config.MessageBufferSize <= 0 {
		config.MessageBufferSize = DefaultMessageBufferSize
	}

	if config.ChannelBufferSize <= 0 {
		config.ChannelBufferSize = DefaultChannelBufferSize
	}

	stopSignal := make(chan struct{})

	requests := newMessageDispatcher(config.ChannelBufferSize)
	subs := newSubscriptions(config.SubscriptionBufferSize)

	client := &RPCClient{
		conn:          conn,
		config:        config,
		requests:      requests,
		subscriptions: subs,
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

	slog.Warn("Close the rpc client...")
	// Send stop signal to the background receiving loop
	close(client.stopSignal)

	err := client.conn.Close()
	if err != nil {
		slog.Error("Close connection", "error", err)
	}

	client.requests.Close()
	client.subscriptions.Close()
}

// Call sends a synchronous RPC call to the server with the specified method and parameters.
// It waits for and returns the response result, or an error if the call fails.
func (client *RPCClient) Call(method string, params any) (json.RawMessage, error) {
	slog.Debug("Call", "method", method, "params", params)
	response, err := client.sendRequest(method, params)
	if err != nil {
		return nil, err
	}

	return response.Result, nil
}

// Subscribe sends a subscription request to the server and returns a Subscription object.
// The subscription can be used to receive notifications from the server for the specified method.
func (client *RPCClient) Subscribe(method string, params any) (*Subscription, error) {
	slog.Debug("Subscribe", "method", method, "params", params)
	response, err := client.sendRequest(method, params)
	if err != nil {
		return nil, err
	}

	if response.Result == nil {
		return nil, InvalidResponseResultError
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
	slog.Debug("Unsubscribe", "method", method, "subID", subID)
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
	slog.Debug("Background loop started")

	newMsgCh := make(chan []byte)
	receiveErrCh := make(chan error)

	// Start listing for new messages
	go func() {
		for {
			msg := make([]byte, client.config.MessageBufferSize)
			n, err := client.conn.Read(msg)
			if err != nil {
				receiveErrCh <- err
				return
			}
			select {
			case <-client.stopSignal:
				return
			case newMsgCh <- msg[:n]:
			}
		}
	}()

	for {
		select {
		case err := <-receiveErrCh:
			slog.Error("Read a new msg", "error", err)
			return err
		case <-stopSignal:
			slog.Debug("Background receiving loop stopped")
			return nil
		case msg := <-newMsgCh:
			err := client.handleNewMsg(msg)
			if err != nil {
				slog.Error("Handle a msg", "error", err)
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
			return InvalidResponseIDError
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

		slog.Debug("<--", "notification", notification.String())

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
		return response, ClientIsDisconnectedError
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

	_, err = client.conn.Write(reqJSON)
	if err != nil {
		return response, err
	}

	slog.Debug("-->", "request", req.String())

	rx_ch := client.requests.Register(id)
	defer client.requests.Unregister(id)

	// Waits the response, it fails and return error if it exceed the timeout
	select {
	case response = <-rx_ch:
	case <-client.stopSignal:
		return response, ClientIsDisconnectedError
	case <-time.After(time.Duration(client.config.Timeout) * time.Millisecond):
		return response, TimeoutError
	}

	err = validateResponse(&response, id)
	if err != nil {
		return response, err
	}

	slog.Debug("<--", "response", response.String())

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
			return InvalidResponseIDError
		}
	}

	return nil
}
