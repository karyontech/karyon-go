package client

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSubscriptionsSubscribe(t *testing.T) {
	bufSize := 100
	subs := newSubscriptions(bufSize)

	var receivedNotifications atomic.Int32

	var wg sync.WaitGroup

	runSubNotify := func(sub *Subscription) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range bufSize {
				b, err := json.Marshal(i)
				if err != nil {
					t.Errorf("json.Marshal failed: %v", err)
				}
				err = sub.notify(b)
				if err != nil {
					t.Errorf("sub.Notify failed: %v", err)
				}
			}
		}()
	}

	runSubRecv := func(sub *Subscription) {
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
				receivedNotifications.Add(1)
				i += 1
				if i == bufSize {
					break
				}
			}
			sub.Close()
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 3 {
			sub := subs.Subscribe(i)
			runSubNotify(sub)
			runSubRecv(sub)
		}
	}()

	wg.Wait()
	expected := int32(bufSize * 3)
	if receivedNotifications.Load() != expected {
		t.Fatalf("expected %d notifications, got %d", expected, receivedNotifications.Load())
	}
}

func TestSubscriptionsUnsubscribe(t *testing.T) {
	bufSize := 100
	subs := newSubscriptions(bufSize)

	var wg sync.WaitGroup

	sub := subs.Subscribe(1)
	subs.Unsubscribe(1)

	_, ok := <-sub.Recv()
	if ok {
		t.Fatal("expected channel to be closed")
	}

	b, err := json.Marshal(1)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	err = sub.notify(b)
	if err == nil {
		t.Fatal("expected error when notifying closed subscription")
	}
	if !errors.Is(err, errSubscriptionIsClosed) {
		t.Fatalf("expected errSubscriptionIsClosed, got %v", err)
	}

	wg.Wait()
}
