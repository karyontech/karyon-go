package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	log "github.com/sirupsen/logrus"

	"github.com/karyontech/karyon-go/jsonrpc/message"
)

var (
	SubscriptionIsClosedErr = errors.New("Subscription is closed")
	// TODO
	receivedStopSignalErr   = errors.New("Received stop signal")
)

// Subscription A subscription established when the client's subscribe to a method
type Subscription struct {
	ch         chan json.RawMessage
	ID         message.SubscriptionID
	queue      *ConcurrentQueue[json.RawMessage]
	stopSignal chan struct{}
	isClosed   atomic.Bool
}

// newSubscription Creates a new Subscription
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

// Recv Receives a new notification.
func (s *Subscription) Recv() <-chan json.RawMessage {
	return s.ch
}

// startBackgroundJob starts waiting for the queue to receive new items.
// It stops when it receives a stop signal.
func (s *Subscription) startBackgroundJob() {
	go func() {
		logger := log.WithField("Subscription", s.ID)
		for {
			msg, err := s.queue.Pop()
			if err != nil {
				logger.WithError(err).Error("Background job stopped")
				return
			}
			select {
			case <-s.stopSignal:
				logger.Debug("Background job stopped: %w", receivedStopSignalErr)
				return
			case s.ch <- msg:
			}
		}
	}()
}

// Notify adds a new notification to the queue.
func (s *Subscription) Notify(nt json.RawMessage) error {
	if s.isClosed.Load() {
		return SubscriptionIsClosedErr
	}
	if err := s.queue.Push(nt); err != nil {
		return fmt.Errorf("Unable to push new notification: %w", err)
	}
	return nil
}

// Close Terminates the subscription, closes the queue, and closes channels.
func (s *Subscription) Close() {
	if !s.isClosed.CompareAndSwap(false, true) {
		return
	}
	close(s.stopSignal)
	close(s.ch)
	s.queue.Close()
}
