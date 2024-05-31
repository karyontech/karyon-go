package client

import (
	"fmt"
	"sync"
)

// channels is a generic structure that holds a map of keys and channels.
// It is protected by mutex
type channels[K comparable, V any] struct {
	sync.Mutex
	chans      map[K]chan<- V
	bufferSize int
}

// newChannels creates a new channels
func newChannels[K comparable, V any](bufferSize int) channels[K, V] {
	chans := make(map[K]chan<- V)
	return channels[K, V]{
		chans:      chans,
		bufferSize: bufferSize,
	}
}

// add adds a new channel and returns the receiving channel
func (c *channels[K, V]) add(key K) <-chan V {
	c.Lock()
	defer c.Unlock()

	ch := make(chan V, c.bufferSize)
	c.chans[key] = ch
	return ch
}

// length returns the number of channels
func (c *channels[K, V]) length() int {
	c.Lock()
	defer c.Unlock()

	return len(c.chans)
}

// notify notifies the channel with the given key
func (c *channels[K, V]) notify(key K, msg V) error {
	c.Lock()
	defer c.Unlock()

	if ch, ok := c.chans[key]; ok {
		ch <- msg
		return nil
	}

	return fmt.Errorf("Channel not found")
}

// remove removes and returns the channel.
func (c *channels[K, V]) remove(key K) chan<- V {
	c.Lock()
	defer c.Unlock()

	if ch, ok := c.chans[key]; ok {
		delete(c.chans, key)
		return ch
	}

	return nil
}

// clear close all the channels and remove them from the map
func (c *channels[K, V]) clear() {
	c.Lock()
	defer c.Unlock()

	for k, ch := range c.chans {
		close(ch)
		delete(c.chans, k)
	}
}
