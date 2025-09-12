package util 

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/karyontech/karyon-go/jsonrpc/message"
)

var (
	subscriptionNotFoundErr = errors.New("Subscription not found")
)

// Subscriptions Is a structure that holds a map of subscription IDs and
// Subscriptions
type Subscriptions struct {
	sync.Mutex
	subs       map[message.SubscriptionID]*Subscription
	bufferSize int
}

// NewSubscriptions Creates a new Subscriptions
func NewSubscriptions(bufferSize int) *Subscriptions {
	subs := make(map[message.SubscriptionID]*Subscription)
	return &Subscriptions{
		subs:       subs,
		bufferSize: bufferSize,
	}
}

// Subscribe Subscribes and returns a Subscription.
func (c *Subscriptions) Subscribe(key message.SubscriptionID) *Subscription {
	c.Lock()
	defer c.Unlock()

	sub := NewSubscription(key, c.bufferSize)
	c.subs[key] = sub
	return sub
}

// Notify Notifies the msg the subscription with the given id
func (c *Subscriptions) Notify(key message.SubscriptionID, msg json.RawMessage) error {
	c.Lock()
	defer c.Unlock()

	sub, ok := c.subs[key]

	if !ok {
		return subscriptionNotFoundErr
	}

	err := sub.Notify(msg)

	if err != nil {
		return err
	}

	return nil
}

// Unsubscribe Unsubscribe from the subscription with the provided id
func (c *Subscriptions) Unsubscribe(key message.SubscriptionID) {
	c.Lock()
	defer c.Unlock()
	if sub, ok := c.subs[key]; ok {
		sub.Close()
		delete(c.subs, key)
	}
}

// Close Stops all the Subscriptions and remove them from the map
func (c *Subscriptions) Close() {
	c.Lock()
	defer c.Unlock()

	for _, sub := range c.subs {
		sub.Close()
	}
	c.subs = nil
}
