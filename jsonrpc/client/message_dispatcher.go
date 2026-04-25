package client

import (
	"errors"
	"sync"

	"github.com/karyontech/karyon-jsonrpc-go/jsonrpc/message"
)

const (
	// DefaultChannelBufferSize is the default buffer size for response channels.
	DefaultChannelBufferSize = 1
)

var errChannelNotFound = errors.New("Request channel not found")

// messageDispatcher holds a map of request IDs to response channels,
// protected by a mutex.
type messageDispatcher struct {
	sync.Mutex
	chans      map[message.RequestID]chan<- message.Response
	bufferSize int
}

// newMessageDispatcher creates a new messageDispatcher.
// If bufferSize is 0 or negative, the default buffer size is used.
func newMessageDispatcher(bufferSize int) *messageDispatcher {
	chans := make(map[message.RequestID]chan<- message.Response)

	size := DefaultChannelBufferSize
	if bufferSize > 0 {
		size = bufferSize
	}

	return &messageDispatcher{
		chans:      chans,
		bufferSize: size,
	}
}

// Register creates and registers a new response channel for the given request ID.
func (c *messageDispatcher) Register(key message.RequestID) <-chan message.Response {
	c.Lock()
	defer c.Unlock()

	ch := make(chan message.Response, c.bufferSize)
	c.chans[key] = ch
	return ch
}

// Dispatch sends the response to the channel associated with the given request ID.
// It returns an error if no channel is found for the request ID.
func (c *messageDispatcher) Dispatch(key message.RequestID, res message.Response) error {
	c.Lock()
	defer c.Unlock()

	if ch, ok := c.chans[key]; ok {
		ch <- res
	} else {
		return errChannelNotFound
	}

	return nil
}

// Unregister removes and closes the response channel for the given request ID.
func (c *messageDispatcher) Unregister(key message.RequestID) {
	c.Lock()
	defer c.Unlock()

	if ch, ok := c.chans[key]; ok {
		close(ch)
		delete(c.chans, key)
	}
}

// Close terminates all active request channels and cleans up the dispatcher map.
func (c *messageDispatcher) Close() {
	c.Lock()
	defer c.Unlock()

	for _, ch := range c.chans {
		close(ch)
	}
	c.chans = nil
}
