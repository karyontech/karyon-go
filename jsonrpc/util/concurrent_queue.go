package util  

import (
	"errors"
	"sync"
	"sync/atomic"
)

// queue A concurrent queue.
type ConcurrentQueue[T any] struct {
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

// NewConcurrentQueue creates a new concurrent queue with the specified buffer size.
// It initializes the queue with empty items slice and sets up synchronization primitives.
func NewConcurrentQueue[T any](bufferSize int) *ConcurrentQueue[T] {
	q := &ConcurrentQueue[T]{
		bufferSize: bufferSize,
		items:      make([]T, 0),
		stopSignal: make(chan struct{}),
	}
	q.cond = sync.NewCond(&q.lock)
	return q
}

// Push adds a new item to the end of the queue.
// It returns an error if the queue is full or closed.
func (q *ConcurrentQueue[T]) Push(item T) error {
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

// Pop waits for and removes the first element from the queue.
// It blocks until an item is available or the queue is closed, then returns the item.
func (q *ConcurrentQueue[T]) Pop() (T, error) {
	var t T
	if q.isClosed.Load() {
		return t, queueIsClosedErr
	}
	q.lock.Lock()
	defer q.lock.Unlock()

	// Wait for an item to be available or for a stop signal.
	// This ensures that the waiting stops once the queue is cleared.
	for len(q.items) == 0 {
		select {
		case <-q.stopSignal:
			return t, queueIsClosedErr
		default:
			q.cond.Wait()
		}
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item, nil
}

// Close terminates the queue and releases all resources.
// It marks the queue as closed, clears all items, and wakes up any waiting goroutines.
func (q *ConcurrentQueue[T]) Close() {
	if !q.isClosed.CompareAndSwap(false, true) {
		return
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	close(q.stopSignal)
	q.items = nil
	q.cond.Broadcast()
}
