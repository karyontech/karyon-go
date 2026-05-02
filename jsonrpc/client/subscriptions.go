package client

import (
	"encoding/json"
	"sync"

	"github.com/karyontech/karyon-jsonrpc-go/jsonrpc/message"
)

// subscriptions holds a map of subscription IDs to Subscriptions.
type subscriptions struct {
	sync.Mutex
	subs       map[message.SubscriptionID]*Subscription
	bufferSize int
}

// newSubscriptions creates a new subscriptions manager.
// bufferSize is applied to each child Subscription.
func newSubscriptions(bufferSize int) *subscriptions {
	subs := make(map[message.SubscriptionID]*Subscription)
	return &subscriptions{
		subs:       subs,
		bufferSize: bufferSize,
	}
}

// Subscribe creates a new Subscription with the given ID and registers it.
func (c *subscriptions) Subscribe(key message.SubscriptionID) *Subscription {
	c.Lock()
	defer c.Unlock()

	sub := newSubscription(key, c.bufferSize)
	c.subs[key] = sub
	return sub
}

// Notify forwards a message to the subscription with the specified ID.
func (c *subscriptions) Notify(key message.SubscriptionID, msg json.RawMessage) error {
	c.Lock()
	defer c.Unlock()

	sub, ok := c.subs[key]

	if !ok {
		return errSubscriptionNotFound
	}

	if err := sub.notify(msg); err != nil {
		return err
	}

	return nil
}

// Unsubscribe removes and closes the subscription with the specified ID.
func (c *subscriptions) Unsubscribe(key message.SubscriptionID) {
	c.Lock()
	defer c.Unlock()
	if sub, ok := c.subs[key]; ok {
		sub.Close()
		delete(c.subs, key)
	}
}

// Close terminates all active subscriptions and clears the map.
func (c *subscriptions) Close() {
	c.Lock()
	defer c.Unlock()

	for _, sub := range c.subs {
		sub.Close()
	}
	c.subs = nil
}
