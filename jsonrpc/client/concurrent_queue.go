package client

import (
	"errors"
	"sync"
	"sync/atomic"
)

// queue A concurrent queue.
type queue[T any] struct {
	lock       sync.Mutex
	cond       *sync.Cond
	items      []T
	bufferSize int
	stopSignal chan struct{}
	isClosed   atomic.Bool
}

var (
	queueIsFullErr   = errors.New("Queue is full")
	queueIsClosedErr = errors.New("Queue is closed")
)

// newQueue creates a new queue with the specified buffer size.
func newQueue[T any](bufferSize int) *queue[T] {
	q := &queue[T]{
		bufferSize: bufferSize,
		items:      make([]T, 0),
		stopSignal: make(chan struct{}),
	}
	q.cond = sync.NewCond(&q.lock)
	return q
}

// push Adds a new item to the queue and returns an error if the queue
// is full.
func (q *queue[T]) push(item T) error {
	if q.isClosed.Load() {
		return queueIsClosedErr
	}

	q.lock.Lock()
	defer q.lock.Unlock()
	if len(q.items) >= q.bufferSize {
		return queueIsFullErr
	}

	q.items = append(q.items, item)
	q.cond.Signal()
	return nil
}

// pop waits for and removes the first element from the queue, then returns it.
func (q *queue[T]) pop() (T, bool) {
	var t T
	if q.isClosed.Load() {
		return t, false
	}
	q.lock.Lock()
	defer q.lock.Unlock()

	// Wait for an item to be available or for a stop signal.
	// This ensures that the waiting stops once the queue is cleared.
	for len(q.items) == 0 {
		select {
		case <-q.stopSignal:
			return t, false
		default:
			q.cond.Wait()
		}
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

// clear Clears all elements from the queue.
func (q *queue[T]) clear() {
	if !q.isClosed.CompareAndSwap(false, true) {
		return
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	close(q.stopSignal)
	q.items = nil
	q.cond.Broadcast()
}
