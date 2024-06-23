package client

import (
	"errors"
	"sync"

	"github.com/karyontech/karyon-go/jsonrpc/message"
)

var (
	requestChannelNotFoundErr = errors.New("Request channel not found")
)

// messageDispatcher Is a structure that holds a map of request IDs and
// channels, and it is protected by mutex
type messageDispatcher struct {
	sync.Mutex
	chans map[message.RequestID]chan<- message.Response
}

// newMessageDispatcher Creates a new messageDispatcher
func newMessageDispatcher() *messageDispatcher {
	chans := make(map[message.RequestID]chan<- message.Response)
	return &messageDispatcher{
		chans: chans,
	}
}

// register Registers a new request channel with the given id. It returns a
// channel for receiving response.
func (c *messageDispatcher) register(key message.RequestID) <-chan message.Response {
	c.Lock()
	defer c.Unlock()

	ch := make(chan message.Response)
	c.chans[key] = ch
	return ch
}

// dispatch Disptaches the response to the channel with the given request id
func (c *messageDispatcher) dispatch(key message.RequestID, res message.Response) error {
	c.Lock()
	defer c.Unlock()

	if ch, ok := c.chans[key]; ok {
		ch <- res
	} else {
		return requestChannelNotFoundErr
	}

	return nil
}

// unregister Unregisters the request with the provided id
func (c *messageDispatcher) unregister(key message.RequestID) {
	c.Lock()
	defer c.Unlock()

	if ch, ok := c.chans[key]; ok {
		close(ch)
		delete(c.chans, key)
	}
}

// clear Closes all the request channels and remove them from the map
func (c *messageDispatcher) clear() {
	c.Lock()
	defer c.Unlock()

	for _, ch := range c.chans {
		close(ch)
	}
	c.chans = nil
}
