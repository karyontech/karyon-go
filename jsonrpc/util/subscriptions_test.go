package util

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscriptionsSubscribe(t *testing.T) {
	bufSize := 100
	subs := NewSubscriptions(bufSize)

	var receivedNotifications atomic.Int32

	var wg sync.WaitGroup

	runSubNotify := func(sub *Subscription) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range bufSize {
				b, err := json.Marshal(i)
				assert.Nil(t, err)
				err = sub.Notify(b)
				assert.Nil(t, err)
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
				assert.Nil(t, err)
				assert.Equal(t, v, i)
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
	assert.Equal(t, receivedNotifications.Load(), int32(bufSize*3))
}

func TestSubscriptionsUnsubscribe(t *testing.T) {
	bufSize := 100
	subs := NewSubscriptions(bufSize)

	var wg sync.WaitGroup

	sub := subs.Subscribe(1)
	subs.Unsubscribe(1)

	_, ok := <-sub.Recv()
	assert.False(t, ok)

	b, err := json.Marshal(1)
	assert.Nil(t, err)
	err = sub.Notify(b)
	if assert.Error(t, err) {
		assert.ErrorIs(t, err, SubscriptionIsClosedError)
	}

	wg.Wait()
}
