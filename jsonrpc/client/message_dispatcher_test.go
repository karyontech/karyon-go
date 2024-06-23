package client

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/karyontech/karyon-go/jsonrpc/message"
	"github.com/stretchr/testify/assert"
)

func TestDispatchToChannel(t *testing.T) {
	messageDispatcher := newMessageDispatcher()

	req1 := "1"
	rx := messageDispatcher.register(req1)

	req2 := "2"
	rx2 := messageDispatcher.register(req2)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			res := message.Response{ID: &req1}
			err := messageDispatcher.dispatch(req1, res)
			assert.Nil(t, err)
		}

		messageDispatcher.unregister(req1)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			res := message.Response{ID: &req2}
			err := messageDispatcher.dispatch(req2, res)
			assert.Nil(t, err)
		}

		messageDispatcher.unregister(req2)
	}()

	var receivedItem atomic.Int32

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range rx {
			receivedItem.Add(1)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range rx2 {
			receivedItem.Add(1)
		}
	}()

	wg.Wait()
	assert.Equal(t, receivedItem.Load(), int32(100))
}

func TestUnregisterChannel(t *testing.T) {
	messageDispatcher := newMessageDispatcher()

	req := "1"
	rx := messageDispatcher.register(req)

	messageDispatcher.unregister(req)

	_, ok := <-rx
	assert.False(t, ok, "chan closed")

	err := messageDispatcher.dispatch(req, message.Response{ID: &req})
	assert.NotNil(t, err)
}

func TestClearChannels(t *testing.T) {
	messageDispatcher := newMessageDispatcher()

	req := "1"
	rx := messageDispatcher.register(req)

	messageDispatcher.clear()

	_, ok := <-rx
	assert.False(t, ok, "chan closed")

	err := messageDispatcher.dispatch(req, message.Response{ID: &req})
	assert.NotNil(t, err)
}
