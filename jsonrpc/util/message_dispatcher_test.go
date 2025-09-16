package util

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/karyontech/karyon-go/jsonrpc/message"
)

func TestDispatchToChannel(t *testing.T) {
	messageDispatcher := NewMessageDispatcher(1)

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
			if err != nil {
				t.Errorf("Dispatch failed: %v", err)
			}
		}

		messageDispatcher.Unregister(req1)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 50 {
			res := message.Response{ID: &req2}
			err := messageDispatcher.Dispatch(req2, res)
			if err != nil {
				t.Errorf("Dispatch failed: %v", err)
			}
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
	expected := int32(100)
	if receivedItem.Load() != expected {
		t.Fatalf("expected %d received items, got %d", expected, receivedItem.Load())
	}
}

func TestUnregisterChannel(t *testing.T) {
	messageDispatcher := NewMessageDispatcher(1)

	req := "1"
	rx := messageDispatcher.Register(req)

	messageDispatcher.Unregister(req)

	_, ok := <-rx
	if ok {
		t.Fatal("expected channel to be closed")
	}

	err := messageDispatcher.Dispatch(req, message.Response{ID: &req})
	if err == nil {
		t.Fatal("expected error when dispatching to unregistered channel")
	}
}

func TestClearChannels(t *testing.T) {
	messageDispatcher := NewMessageDispatcher(1)

	req := "1"
	rx := messageDispatcher.Register(req)

	messageDispatcher.Close()

	_, ok := <-rx
	if ok {
		t.Fatal("expected channel to be closed")
	}

	err := messageDispatcher.Dispatch(req, message.Response{ID: &req})
	if err == nil {
		t.Fatal("expected error when dispatching to unregistered channel")
	}
}
