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

	"github.com/karyontech/karyon-go/message"
)

const (
	// JsonRPCVersion defines the version of the JSON-RPC protocol being used.
	JsonRPCVersion = "2.0"

	// Default timeout for receiving requests from the server, in milliseconds.
	DefaultTimeout = 3000
)

// RPCClientConfig holds the configuration settings for the RPC client.
type RPCClientConfig struct {
	Timeout int    // Timeout for receiving requests from the server, in milliseconds.
	Addr    string // Address of the RPC server.
}

// RPCClient
type RPCClient struct {
	config        RPCClientConfig
	conn          *websocket.Conn
	request_chans channels[message.RequestID, message.Response]
	subscriptions channels[message.SubscriptionID, json.RawMessage]
	stop_signal   chan struct{}
}

// NewRPCClient creates a new instance of RPCClient with the provided configuration.
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
		conn:          conn,
		config:        config,
		request_chans: newChannels[message.RequestID, message.Response](1),
		subscriptions: newChannels[message.SubscriptionID, json.RawMessage](10),
		stop_signal:   stop_signal,
	}

	go func() {
		if err := client.backgroundReceivingLoop(stop_signal); err != nil {
			client.Close()
		}
	}()

	return client, nil
}

// Close closes the underlying websocket connection and stop the receiving loop.
func (client *RPCClient) Close() error {
	log.Warn("Close the rpc client...")
	client.stop_signal <- struct{}{}

	err := client.conn.Close()
	if err != nil {
		log.WithError(err).Error("Close websocket connection")
	}

	client.request_chans.clear()
	client.subscriptions.clear()
	return nil
}

// Call sends an RPC call to the server with the specified method and parameters.
// It returns the response from the server.
func (client *RPCClient) Call(method string, params any) (*json.RawMessage, error) {
	log.Tracef("Call -> method: %s, params: %v", method, params)
	param_raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	response, err := client.sendRequest(method, param_raw)
	if err != nil {
		return nil, err
	}

	return response.Result, nil
}

// Subscribe sends a subscription request to the server with the specified method and parameters.
// It returns the subscription ID and a channel to receive notifications.
func (client *RPCClient) Subscribe(method string, params any) (message.SubscriptionID, <-chan json.RawMessage, error) {
	log.Tracef("Sbuscribe ->  method: %s, params: %v", method, params)
	param_raw, err := json.Marshal(params)
	if err != nil {
		return 0, nil, err
	}

	response, err := client.sendRequest(method, param_raw)
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

	ch := client.subscriptions.add(subID)

	return subID, ch, nil
}

// Unsubscribe sends an unsubscription request to the server to cancel the given subscription.
func (client *RPCClient) Unsubscribe(method string, subID message.SubscriptionID) error {
	log.Tracef("Unsubscribe -> method: %s, subID: %d", method, subID)
	subIDJSON, err := json.Marshal(subID)
	if err != nil {
		return err
	}

	_, err = client.sendRequest(method, subIDJSON)
	if err != nil {
		return err
	}

	// on success remove the subscription from the map
	client.subscriptions.remove(subID)

	return nil
}

// backgroundReceivingLoop starts reading new messages from the underlying connection.
func (client *RPCClient) backgroundReceivingLoop(stop_signal <-chan struct{}) error {
	log.Debug("Background loop started")
	for {
		select {
		case <-stop_signal:
			log.Warn("Stopping background receiving loop: received stop signal")
			return nil
		default:
			_, msg, err := client.conn.ReadMessage()
			if err != nil {
				log.WithError(err).Error("Receive a new msg")
				return err
			}

			err = client.handleNewMsg(msg)
			if err != nil {
				log.WithError(err).Error("Handle a new received msg")
			}
		}
	}
}

// handleNewMsg attempts to decode the received message into either a Response
// or Notification struct.
func (client *RPCClient) handleNewMsg(msg []byte) error {
	// try to decode the msg into message.Response
	response := message.Response{}
	decoder := json.NewDecoder(bytes.NewReader(msg))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err == nil {
		if response.ID == nil {
			return fmt.Errorf("Response doesn't have an id")
		}

		if v := client.request_chans.remove(*response.ID); v != nil {
			v <- response
		}

		return nil
	}

	// try to decode the msg into message.Notification
	notification := message.Notification{}
	if err := json.Unmarshal(msg, &notification); err == nil {

		notificationResult := message.NotificationResult{}
		if err := json.Unmarshal(*notification.Params, &notificationResult); err != nil {
			return fmt.Errorf("Failed to unmarshal notification params: %w", err)
		}

		err := client.subscriptions.notify(
			notificationResult.Subscription,
			*notificationResult.Result,
		)
		if err != nil {
			return fmt.Errorf("Notify a subscriber: %w", err)
		}

		log.Debugf("<-- %s", notification.String())

		return nil
	}

	return fmt.Errorf("Receive unexpected msg: %s", msg)
}

// sendRequest sends a request and wait the response
func (client *RPCClient) sendRequest(method string, params []byte) (message.Response, error) {
	id := strconv.Itoa(rand.Int())
	params_raw := json.RawMessage(params)

	req := message.Request{
		JSONRPC: JsonRPCVersion,
		ID:      id,
		Method:  method,
		Params:  &params_raw,
	}

	response := message.Response{}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return response, err
	}

	err = client.conn.WriteMessage(websocket.TextMessage, []byte(string(reqJSON)))
	if err != nil {
		return response, err
	}

	log.Debugf("--> %s", req.String())

	req_chan := client.request_chans.add(id)

	response, err = client.waitResponse(req_chan)
	if err != nil {
		log.WithError(err).Errorf("Receive a response from the server")
		client.request_chans.remove(id)
		return response, err
	}

	err = validateResponse(&response, id)
	if err != nil {
		return response, err
	}

	log.Debugf("<-- %s", response.String())

	return response, nil
}

// waitResponse waits the response, it fails and return error if it exceed the timeout
func (client *RPCClient) waitResponse(ch <-chan message.Response) (message.Response, error) {
	response := message.Response{}
	select {
	case response = <-ch:
		return response, nil
	case <-time.After(time.Duration(client.config.Timeout) * time.Millisecond):
		return response, fmt.Errorf("Timeout error")
	}
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
