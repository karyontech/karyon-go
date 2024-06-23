package client

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/karyontech/karyon-go/jsonrpc/message"
)

var (
	subscriptionNotFoundErr = errors.New("Subscription not found")
)

// subscriptions Is a structure that holds a map of subscription IDs and
// subscriptions
type subscriptions struct {
	sync.Mutex
	subs       map[message.SubscriptionID]*Subscription
	bufferSize int
}

// newSubscriptions Creates a new subscriptions
func newSubscriptions(bufferSize int) *subscriptions {
	subs := make(map[message.SubscriptionID]*Subscription)
	return &subscriptions{
		subs:       subs,
		bufferSize: bufferSize,
	}
}

// subscribe Subscribes and returns a Subscription.
func (c *subscriptions) subscribe(key message.SubscriptionID) *Subscription {
	c.Lock()
	defer c.Unlock()

	sub := newSubscription(key, c.bufferSize)
	c.subs[key] = sub
	return sub
}

// notify Notifies the msg the subscription with the given id
func (c *subscriptions) notify(key message.SubscriptionID, msg json.RawMessage) error {
	c.Lock()
	defer c.Unlock()

	sub, ok := c.subs[key]

	if !ok {
		return subscriptionNotFoundErr
	}

	err := sub.notify(msg)

	if err != nil {
		return err
	}

	return nil
}

// unsubscribe Unsubscribe from the subscription with the provided id
func (c *subscriptions) unsubscribe(key message.SubscriptionID) {
	c.Lock()
	defer c.Unlock()
	if sub, ok := c.subs[key]; ok {
		sub.stop()
		delete(c.subs, key)
	}
}

// clear Stops all the subscriptions and remove them from the map
func (c *subscriptions) clear() {
	c.Lock()
	defer c.Unlock()

	for _, sub := range c.subs {
		sub.stop()
	}
	c.subs = nil
}
