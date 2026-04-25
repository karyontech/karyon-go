package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/karyontech/karyon-jsonrpc-go/jsonrpc/message"
)

var (
	errSubscriptionIsClosed = errors.New("Subscription is closed")
	errSubscriptionNotFound = errors.New("Subscription not found")
)

// Subscription is established when the client subscribes to a method.
type Subscription struct {
	ch         chan json.RawMessage
	ID         message.SubscriptionID
	queue      *concurrentQueue[json.RawMessage]
	stopSignal chan struct{}
	isClosed   atomic.Bool
}

// newSubscription creates a new Subscription and starts its background dispatch job.
func newSubscription(subID message.SubscriptionID, bufferSize int) *Subscription {
	sub := &Subscription{
		ch:         make(chan json.RawMessage),
		ID:         subID,
		queue:      newConcurrentQueue[json.RawMessage](bufferSize),
		stopSignal: make(chan struct{}),
	}
	sub.startBackgroundJob()

	return sub
}

// Recv returns a receive-only channel for reading notifications from the subscription.
func (s *Subscription) Recv() <-chan json.RawMessage {
	return s.ch
}

// startBackgroundJob continuously drains the internal queue and forwards
// items to the subscription channel until a stop signal is received.
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

// notify enqueues a new notification for processing.
// Returns an error if the subscription is closed or the queue is full.
func (s *Subscription) notify(nt json.RawMessage) error {
	if s.isClosed.Load() {
		return errSubscriptionIsClosed
	}
	if err := s.queue.Push(nt); err != nil {
		return fmt.Errorf("Unable to push new notification: %w", err)
	}
	return nil
}

// Close terminates the subscription, stopping the background job and releasing resources.
func (s *Subscription) Close() {
	if !s.isClosed.CompareAndSwap(false, true) {
		return
	}
	close(s.stopSignal)
	close(s.ch)
	s.queue.Close()
}
