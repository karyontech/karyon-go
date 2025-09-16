package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/karyontech/karyon-go/jsonrpc/message"
)

var (
	SubscriptionIsClosedError = errors.New("Subscription is closed")
	SubscriptionNotFoundError = errors.New("Subscription not found")
)

// Subscription A subscription established when the client's subscribe to a method
type Subscription struct {
	ch         chan json.RawMessage
	ID         message.SubscriptionID
	queue      *ConcurrentQueue[json.RawMessage]
	stopSignal chan struct{}
	isClosed   atomic.Bool
}

// NewSubscription creates a new Subscription with the specified subscription ID and buffer size.
// It initializes the subscription channels and queue, and starts a background job to handle notifications.
func NewSubscription(subID message.SubscriptionID, bufferSize int) *Subscription {
	sub := &Subscription{
		ch:         make(chan json.RawMessage),
		ID:         subID,
		queue:      NewConcurrentQueue[json.RawMessage](bufferSize),
		stopSignal: make(chan struct{}),
	}
	sub.startBackgroundJob()

	return sub
}

// Recv returns a receive-only channel for reading notifications from the subscription.
// This channel will receive notifications as they are processed from the internal queue.
func (s *Subscription) Recv() <-chan json.RawMessage {
	return s.ch
}

// startBackgroundJob starts a goroutine that continuously processes notifications from the queue.
// It waits for items to be available in the queue and forwards them to the subscription channel.
// The job stops when it receives a stop signal or encounters an error.
func (s *Subscription) startBackgroundJob() {
	go func() {
		logger := slog.With("Subscription", s.ID)
		for {
			msg, err := s.queue.Pop()
			if err != nil {
				logger.Error("Background job stopped", "error", err)
				return
			}
			select {
			case <-s.stopSignal:
				logger.Debug("Background job stopped receive a stop signal")
				return
			case s.ch <- msg:
			}
		}
	}()
}

// Notify adds a new notification to the subscription's queue for processing.
// It returns an error if the subscription is closed or if the queue is full.
func (s *Subscription) Notify(nt json.RawMessage) error {
	if s.isClosed.Load() {
		return SubscriptionIsClosedError
	}
	if err := s.queue.Push(nt); err != nil {
		return fmt.Errorf("Unable to push new notification: %w", err)
	}
	return nil
}

// Close terminates the subscription by stopping the background job, closing channels, and cleaning up resources.
// It ensures that the subscription can only be closed once using atomic operations.
func (s *Subscription) Close() {
	if !s.isClosed.CompareAndSwap(false, true) {
		return
	}
	close(s.stopSignal)
	close(s.ch)
	s.queue.Close()
}
