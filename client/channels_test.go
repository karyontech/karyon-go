package client

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotifyChannel(t *testing.T) {

	chans := newChannels[int, int](10)

	chanKey := 1
	rx := chans.add(chanKey)

	chanKey2 := 2
	rx2 := chans.add(chanKey2)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for i := 0; i < 50; i++ {
			err := chans.notify(chanKey, i)
			assert.Nil(t, err)
		}

		// drop the channel
		tx := chans.remove(chanKey)
		close(tx)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 0; i < 50; i++ {
			err := chans.notify(chanKey2, i)
			assert.Nil(t, err)
		}

		// drop the channel
		tx := chans.remove(chanKey2)
		close(tx)
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

func TestRemoveChannel(t *testing.T) {

	chans := newChannels[int, int](1)

	chanKey := 1
	rx := chans.add(chanKey)

	tx := chans.remove(chanKey)
	assert.Equal(t, chans.length(), 0, "channels should be empty")

	tx <- 3
	val := <-rx
	assert.Equal(t, val, 3)

	tx = chans.remove(chanKey)
	assert.Nil(t, tx)

	err := chans.notify(chanKey, 1)
	assert.NotNil(t, err)
}

func TestClearChannels(t *testing.T) {

	chans := newChannels[int, int](1)

	chanKey := 1
	rx := chans.add(chanKey)

	chans.clear()
	assert.Equal(t, chans.length(), 0, "channels should be empty")

	_, ok := <-rx
	assert.False(t, ok, "chan closed")

	tx := chans.remove(chanKey)
	assert.Nil(t, tx)

	err := chans.notify(chanKey, 1)
	assert.NotNil(t, err)
}
