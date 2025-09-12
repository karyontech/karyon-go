package util

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/karyontech/karyon-go/jsonrpc/message"
	"github.com/stretchr/testify/assert"
)

func TestDispatchToChannel(t *testing.T) {
	messageDispatcher := NewMessageDispatcher()

	req1 := "1"
	rx := messageDispatcher.Register(req1)

	req2 := "2"
	rx2 := messageDispatcher.Register(req2)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 50 {
			res := message.Response{ID: &req1}
			err := messageDispatcher.Dispatch(req1, res)
			assert.Nil(t, err)
		}

		messageDispatcher.Unregister(req1)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 50 {
			res := message.Response{ID: &req2}
			err := messageDispatcher.Dispatch(req2, res)
			assert.Nil(t, err)
		}

		messageDispatcher.Unregister(req2)
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
	messageDispatcher := NewMessageDispatcher()

	req := "1"
	rx := messageDispatcher.Register(req)

	messageDispatcher.Unregister(req)

	_, ok := <-rx
	assert.False(t, ok, "chan closed")

	err := messageDispatcher.Dispatch(req, message.Response{ID: &req})
	assert.NotNil(t, err)
}

func TestClearChannels(t *testing.T) {
	messageDispatcher := NewMessageDispatcher()

	req := "1"
	rx := messageDispatcher.Register(req)

	messageDispatcher.Close()

	_, ok := <-rx
	assert.False(t, ok, "chan closed")

	err := messageDispatcher.Dispatch(req, message.Response{ID: &req})
	assert.NotNil(t, err)
}
