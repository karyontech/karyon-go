package client

import (
	"fmt"
	"sync"
)

// messageDispatcher Is a generic structure that holds a map of keys and
// channels, and it is protected by mutex
type messageDispatcher[K comparable, V any] struct {
	sync.Mutex
	chans      map[K]chan<- V
	bufferSize int
}

// newMessageDispatcher Creates a new messageDispatcher
func newMessageDispatcher[K comparable, V any](bufferSize int) messageDispatcher[K, V] {
	chans := make(map[K]chan<- V)
	return messageDispatcher[K, V]{
		chans:      chans,
		bufferSize: bufferSize,
	}
}

// register Registers a new channel with a given key. It returns the receiving channel.
func (c *messageDispatcher[K, V]) register(key K) <-chan V {
	c.Lock()
	defer c.Unlock()

	ch := make(chan V, c.bufferSize)
	c.chans[key] = ch
	return ch
}

// length Returns the number of channels
func (c *messageDispatcher[K, V]) length() int {
	c.Lock()
	defer c.Unlock()

	return len(c.chans)
}

// disptach Disptaches the msg to the channel with the given key
func (c *messageDispatcher[K, V]) disptach(key K, msg V) error {
	c.Lock()
	defer c.Unlock()

	if ch, ok := c.chans[key]; ok {
		ch <- msg
		return nil
	}

	return fmt.Errorf("Channel not found")
}

// unregister Unregisters the channel with the provided key
func (c *messageDispatcher[K, V]) unregister(key K) {
	c.Lock()
	defer c.Unlock()
	if ch, ok := c.chans[key]; ok {
		close(ch)
		delete(c.chans, key)
	}
}

// clear Closes all the channels and remove them from the map
func (c *messageDispatcher[K, V]) clear() {
	c.Lock()
	defer c.Unlock()

	for k, ch := range c.chans {
		close(ch)
		delete(c.chans, k)
	}
}
