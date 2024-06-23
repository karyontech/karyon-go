package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

var (
	subscriptionIsClosedErr = errors.New("Subscription is closed")
)

// Subscription A subscription established when the client's subscribe to a method
type Subscription struct {
	ch         chan json.RawMessage
	ID         int
	queue      *queue[json.RawMessage]
	stopSignal chan struct{}
	isClosed   atomic.Bool
}

// newSubscription Creates a new Subscription
func newSubscription(subID int, bufferSize int) *Subscription {
	sub := &Subscription{
		ch:         make(chan json.RawMessage),
		ID:         subID,
		queue:      newQueue[json.RawMessage](bufferSize),
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
			msg, ok := s.queue.pop()
			if !ok {
				logger.Debug("Background job stopped")
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

// notify adds a new notification to the queue.
func (s *Subscription) notify(nt json.RawMessage) error {
	if s.isClosed.Load() {
		return subscriptionIsClosedErr
	}
	if err := s.queue.push(nt); err != nil {
		return fmt.Errorf("Unable to push new notification: %w", err)
	}
	return nil
}

// stop Terminates the subscription, clears the queue, and closes channels.
func (s *Subscription) stop() {
	if !s.isClosed.CompareAndSwap(false, true) {
		return
	}
	close(s.stopSignal)
	close(s.ch)
	s.queue.clear()
}
