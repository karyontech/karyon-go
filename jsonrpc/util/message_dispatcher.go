package util

import (
	"errors"
	"sync"

	"github.com/karyontech/karyon-go/jsonrpc/message"
)

const (
	// The default buffer size for response channels.
	DefaultChannelBufferSize = 1
)

var (
	ChannelNotFoundError = errors.New("Request channel not found")
)

// MessageDispatcher Is a structure that holds a map of request IDs and
// channels, and it is protected by mutex
type MessageDispatcher struct {
	sync.Mutex
	chans      map[message.RequestID]chan<- message.Response
	bufferSize int
}

// NewMessageDispatcher creates a new MessageDispatcher with an empty channel map.
// It initializes the internal map for storing request ID to response channel mappings.
// If bufferSize is 0 or negative, it uses the default buffer size.
func NewMessageDispatcher(bufferSize int) *MessageDispatcher {
	chans := make(map[message.RequestID]chan<- message.Response)

	size := DefaultChannelBufferSize
	if bufferSize > 0 {
		size = bufferSize
	}

	return &MessageDispatcher{
		chans:      chans,
		bufferSize: size,
	}
}

// Register creates and registers a new response channel for the given request ID.
// It returns a receive-only channel that will receive the response when it arrives.
func (c *MessageDispatcher) Register(key message.RequestID) <-chan message.Response {
	c.Lock()
	defer c.Unlock()

	ch := make(chan message.Response, c.bufferSize)
	c.chans[key] = ch
	return ch
}

// Dispatch sends the response to the channel associated with the given request ID.
// It returns an error if no channel is found for the request ID.
func (c *MessageDispatcher) Dispatch(key message.RequestID, res message.Response) error {
	c.Lock()
	defer c.Unlock()

	if ch, ok := c.chans[key]; ok {
		ch <- res
	} else {
		return ChannelNotFoundError
	}

	return nil
}

// Unregister removes and closes the response channel for the given request ID.
// It safely handles the case where the request ID doesn't exist.
func (c *MessageDispatcher) Unregister(key message.RequestID) {
	c.Lock()
	defer c.Unlock()

	if ch, ok := c.chans[key]; ok {
		close(ch)
		delete(c.chans, key)
	}
}

// Close terminates all active request channels and cleans up the dispatcher map.
// It ensures all resources are properly released when the dispatcher is no longer needed.
func (c *MessageDispatcher) Close() {
	c.Lock()
	defer c.Unlock()

	for _, ch := range c.chans {
		close(ch)
	}
	c.chans = nil
}
