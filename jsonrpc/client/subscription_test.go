package client

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscriptionFullQueue(t *testing.T) {
	bufSize := 100
	sub := newSubscription(1, bufSize)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer sub.stop()
		for i := 0; i < bufSize+10; i++ {
			b, err := json.Marshal(i)
			assert.Nil(t, err)
			err = sub.notify(b)
			if i > bufSize {
				if assert.Error(t, err) {
					assert.ErrorIs(t, err, queueIsFullErr)
				}
			}
		}
	}()

	wg.Wait()
}

func TestSubscriptionRecv(t *testing.T) {
	bufSize := 100
	sub := newSubscription(1, bufSize)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < bufSize; i++ {
			b, err := json.Marshal(i)
			assert.Nil(t, err)
			err = sub.notify(b)
			assert.Nil(t, err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for nt := range sub.Recv() {
			var v int
			err := json.Unmarshal(nt, &v)
			assert.Nil(t, err)
			assert.Equal(t, v, i)
			i += 1
			if i == bufSize {
				break
			}
		}
	}()

	wg.Wait()
}

func TestSubscriptionStop(t *testing.T) {
	sub := newSubscription(1, 10)

	sub.stop()

	_, ok := <-sub.Recv()
	assert.False(t, ok)

	b, err := json.Marshal(1)
	assert.Nil(t, err)
	err = sub.notify(b)
	if assert.Error(t, err) {
		assert.ErrorIs(t, err, subscriptionIsClosedErr)
	}
}
