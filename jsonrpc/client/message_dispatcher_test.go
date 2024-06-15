package client

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDispatchToChannel(t *testing.T) {
	messageDispatcher := newMessageDispatcher[int, int](10)

	chanKey := 1
	rx := messageDispatcher.register(chanKey)

	chanKey2 := 2
	rx2 := messageDispatcher.register(chanKey2)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for i := 0; i < 50; i++ {
			err := messageDispatcher.dispatch(chanKey, i)
			assert.Nil(t, err)
		}

		messageDispatcher.unregister(chanKey)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 0; i < 50; i++ {
			err := messageDispatcher.dispatch(chanKey2, i)
			assert.Nil(t, err)
		}

		messageDispatcher.unregister(chanKey2)
		wg.Done()
	}()

	var receivedItem atomic.Int32

	wg.Add(1)
	go func() {
		for range rx {
			receivedItem.Add(1)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for range rx2 {
			receivedItem.Add(1)
		}
		wg.Done()
	}()

	wg.Wait()
	assert.Equal(t, receivedItem.Load(), int32(100))
}

func TestUnregisterChannel(t *testing.T) {
	messageDispatcher := newMessageDispatcher[int, int](1)

	chanKey := 1
	rx := messageDispatcher.register(chanKey)

	messageDispatcher.unregister(chanKey)
	assert.Equal(t, messageDispatcher.length(), 0, "channels should be empty")

	_, ok := <-rx
	assert.False(t, ok, "chan closed")

	err := messageDispatcher.dispatch(chanKey, 1)
	assert.NotNil(t, err)
}

func TestClearChannels(t *testing.T) {
	messageDispatcher := newMessageDispatcher[int, int](1)

	chanKey := 1
	rx := messageDispatcher.register(chanKey)

	messageDispatcher.clear()
	assert.Equal(t, messageDispatcher.length(), 0, "channels should be empty")

	_, ok := <-rx
	assert.False(t, ok, "chan closed")

	err := messageDispatcher.dispatch(chanKey, 1)
	assert.NotNil(t, err)
}
