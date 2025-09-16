package util

import (
	"encoding/json"
	"sync"

	"github.com/karyontech/karyon-go/jsonrpc/message"
)

// Subscriptions Is a structure that holds a map of subscription IDs and
// Subscriptions
type Subscriptions struct {
	sync.Mutex
	subs       map[message.SubscriptionID]*Subscription
	bufferSize int
}

// NewSubscriptions creates a new Subscriptions manager with the specified buffer size for each subscription.
// It initializes an empty map to store subscription IDs and their corresponding Subscription objects.
func NewSubscriptions(bufferSize int) *Subscriptions {
	subs := make(map[message.SubscriptionID]*Subscription)
	return &Subscriptions{
		subs:       subs,
		bufferSize: bufferSize,
	}
}

// Subscribe creates a new subscription with the given ID and adds it to the manager.
// It returns the newly created Subscription object that can be used to receive notifications.
func (c *Subscriptions) Subscribe(key message.SubscriptionID) *Subscription {
	c.Lock()
	defer c.Unlock()

	sub := NewSubscription(key, c.bufferSize)
	c.subs[key] = sub
	return sub
}

// Notify sends a message to the subscription with the specified ID.
// It returns an error if the subscription is not found or if the notification fails.
func (c *Subscriptions) Notify(key message.SubscriptionID, msg json.RawMessage) error {
	c.Lock()
	defer c.Unlock()

	sub, ok := c.subs[key]

	if !ok {
		return SubscriptionNotFoundError
	}

	err := sub.Notify(msg)

	if err != nil {
		return err
	}

	return nil
}

// Unsubscribe removes and closes the subscription with the specified ID.
// It safely handles the case where the subscription doesn't exist.
func (c *Subscriptions) Unsubscribe(key message.SubscriptionID) {
	c.Lock()
	defer c.Unlock()
	if sub, ok := c.subs[key]; ok {
		sub.Close()
		delete(c.subs, key)
	}
}

// Close terminates all active subscriptions and cleans up the subscription map.
// It ensures all resources are properly released when the manager is no longer needed.
func (c *Subscriptions) Close() {
	c.Lock()
	defer c.Unlock()

	for _, sub := range c.subs {
		sub.Close()
	}
	c.subs = nil
}
