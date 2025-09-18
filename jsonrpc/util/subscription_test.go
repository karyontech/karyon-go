package util

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

func TestSubscriptionFullQueue(t *testing.T) {
	bufSize := 100
	sub := NewSubscription(1, bufSize)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer sub.Close()
		for i := 0; i < bufSize+10; i++ {
			b, err := json.Marshal(i)
			if err != nil {
				t.Errorf("json.Marshal failed: %v", err)
			}
			err = sub.Notify(b)
			if i > bufSize {
				if err == nil {
					t.Error("expected error when queue is full")
				}
				if !errors.Is(err, QueueIsFullError) {
					t.Errorf("expected QueueIsFullError, got %v", err)
				}
			}
		}
	}()

	wg.Wait()
}

func TestSubscriptionRecv(t *testing.T) {
	bufSize := 100
	sub := NewSubscription(1, bufSize)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range bufSize {
			b, err := json.Marshal(i)
			if err != nil {
				t.Errorf("json.Marshal failed: %v", err)
			}
			err = sub.Notify(b)
			if err != nil {
				t.Errorf("sub.Notify failed: %v", err)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for nt := range sub.Recv() {
			var v int
			err := json.Unmarshal(nt, &v)
			if err != nil {
				t.Errorf("json.Unmarshal failed: %v", err)
			}
			if v != i {
				t.Errorf("expected %d, got %d", i, v)
			}
			i += 1
			if i == bufSize {
				break
			}
		}
	}()

	wg.Wait()
}

func TestSubscriptionClose(t *testing.T) {
	sub := NewSubscription(1, 10)

	sub.Close()

	_, ok := <-sub.Recv()
	if ok {
		t.Fatal("expected channel to be closed")
	}

	b, err := json.Marshal(1)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	err = sub.Notify(b)
	if err == nil {
		t.Fatal("expected error when notifying closed subscription")
	}
	if !errors.Is(err, SubscriptionIsClosedError) {
		t.Fatalf("expected SubscriptionIsClosedError, got %v", err)
	}
}
