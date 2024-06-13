package message

import (
	"encoding/json"
	"fmt"
)

// RequestID is used to identify a request.
type RequestID = string

// SubscriptionID is used to identify a subscription.
type SubscriptionID = int

// Request represents a JSON-RPC request message.
// It includes the JSON-RPC version, an identifier for the request, the method
// to be invoked, and optional parameters.
type Request struct {
	JSONRPC string           `json:"jsonrpc"`          // JSON-RPC version, typically "2.0".
	ID      RequestID        `json:"id"`               // Unique identifier for the request, can be a number or a string.
	Method  string           `json:"method"`           // The name of the method to be invoked.
	Params  *json.RawMessage `json:"params,omitempty"` // Optional parameters for the method.
}

// Response represents a JSON-RPC response message.
// It includes the JSON-RPC version, an identifier matching the request, the result of the request, and an optional error.
type Response struct {
	JSONRPC string           `json:"jsonrpc"`          // JSON-RPC version, typically "2.0".
	ID      *RequestID       `json:"id,omitempty"`     // Unique identifier matching the request ID, can be null for notifications.
	Result  *json.RawMessage `json:"result,omitempty"` // Result of the request if it was successful.
	Error   *Error           `json:"error,omitempty"`  // Error object if the request failed.
}

// Notification represents a JSON-RPC notification message.
type Notification struct {
	JSONRPC string           `json:"jsonrpc"`          // JSON-RPC version, typically "2.0".
	Method  string           `json:"method"`           // The name of the method to be invoked.
	Params  *json.RawMessage `json:"params,omitempty"` // Optional parameters for the method.
}

// NotificationResult represents the result of a subscription notification.
// It includes the result and the subscription ID that triggered the notification.
type NotificationResult struct {
	Result       *json.RawMessage `json:"result,omitempty"` // Result data of the notification.
	Subscription SubscriptionID   `json:"subscription"`     // ID of the subscription that triggered the notification.
}

// Error represents an error in a JSON-RPC response.
// It includes an error code, a message, and optional additional data.
type Error struct {
	Code    int              `json:"code"`           // Error code indicating the type of error.
	Message string           `json:"message"`        // Human-readable error message.
	Data    *json.RawMessage `json:"data,omitempty"` // Optional additional data about the error.
}

func (req *Request) String() string {
	return fmt.Sprintf("{JSONRPC: %s, ID: %s, METHOD: %s, PARAMS: %s}", req.JSONRPC, req.ID, req.Method, *req.Params)
}

func (res *Response) String() string {
	return fmt.Sprintf("{JSONRPC: %s, ID: %s, RESULT: %s, ERROR: %v}", res.JSONRPC, *res.ID, *res.Result, res.Error)
}

func (nt *Notification) String() string {
	return fmt.Sprintf("{JSONRPC: %s, METHOD: %s, PARAMS: %s}", nt.JSONRPC, nt.Method, *nt.Params)
}

func (err *Error) String() string {
	return fmt.Sprintf("{CODE: %d, MESSAGE: %s, DATA: %b}", err.Code, err.Message, err.Data)
}
