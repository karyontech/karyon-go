package util

import (
	"errors"
	"sync"

	"github.com/karyontech/karyon-go/jsonrpc/message"
)

var (
	requestChannelNotFoundErr = errors.New("Request channel not found")
)

// MessageDispatcher Is a structure that holds a map of request IDs and
// channels, and it is protected by mutex
type MessageDispatcher struct {
	sync.Mutex
	chans map[message.RequestID]chan<- message.Response
}

// NewMessageDispatcher Creates a new MessageDispatcher
func NewMessageDispatcher() *MessageDispatcher {
	chans := make(map[message.RequestID]chan<- message.Response)
	return &MessageDispatcher{
		chans: chans,
	}
}

// Register Registers a new request channel with the given id. It returns a
// channel for receiving response.
func (c *MessageDispatcher) Register(key message.RequestID) <-chan message.Response {
	c.Lock()
	defer c.Unlock()

	ch := make(chan message.Response)
	c.chans[key] = ch
	return ch
}

// Dispatch Disptaches the response to the channel with the given request id
func (c *MessageDispatcher) Dispatch(key message.RequestID, res message.Response) error {
	c.Lock()
	defer c.Unlock()

	if ch, ok := c.chans[key]; ok {
		ch <- res
	} else {
		return requestChannelNotFoundErr
	}

	return nil
}

// Unregister Unregisters the request with the provided id
func (c *MessageDispatcher) Unregister(key message.RequestID) {
	c.Lock()
	defer c.Unlock()

	if ch, ok := c.chans[key]; ok {
		close(ch)
		delete(c.chans, key)
	}
}

// Close Closes all the request channels and remove them from the map
func (c *MessageDispatcher) Close() {
	c.Lock()
	defer c.Unlock()

	for _, ch := range c.chans {
		close(ch)
	}
	c.chans = nil
}
