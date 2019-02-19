package ps

import (
	"context"

	"gx/ipfs/QmepvmmYNM6q4RaUiwEikQFhgMFHXg2PLhx2E9iaRd3jmS/go-libp2p-pubsub"
)

// Subscriber subscribes to pubsub topics
type Subscriber struct {
	pubsub *pubsub.PubSub
}

// Subscription is a handle to a pubsub subscription.
// This matches part of the interface to a libp2p.pubsub.Subscription.
type Subscription interface {
	// Topic returns this subscription's topic name
	Topic() string
	// Next returns the next message from this subscription
	Next(ctx context.Context) (*pubsub.Message, error)
	// Cancel cancels this subscription
	Cancel()
}

// NewSubscriber builds a new subscriber
func NewSubscriber(sub *pubsub.PubSub) *Subscriber {
	return &Subscriber{pubsub: sub}
}

// Subscribe subscribes to a pubsub topic
func (s *Subscriber) Subscribe(topic string, opts ...pubsub.SubOpt) (Subscription, error) {
	return s.pubsub.Subscribe(topic, opts...)
}
