package pubsub

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	libp2p "github.com/libp2p/go-libp2p-pubsub"
)

// Subscriber subscribes to pubsub topics
type Subscriber struct {
	pubsub *libp2p.PubSub
}

// Message defines the common interface for go-filecoin message consumers.
// It's a subset of the go-libp2p-pubsub/pubsub.go Message type.
type Message interface {
	GetFrom() peer.ID
	GetData() []byte
}

// Subscription is a handle to a pubsub subscription.
// This matches part of the interface to a libp2p.pubsub.Subscription.
type Subscription interface {
	// Topic returns this subscription's topic name
	Topic() string
	// Next returns the next message from this subscription
	Next(ctx context.Context) (Message, error)
	// Cancel cancels this subscription
	Cancel()
}

// NewSubscriber builds a new subscriber
func NewSubscriber(sub *libp2p.PubSub) *Subscriber {
	return &Subscriber{pubsub: sub}
}

// Subscribe subscribes to a pubsub topic
func (s *Subscriber) Subscribe(topic string) (Subscription, error) {
	sub, e := s.pubsub.Subscribe(topic)
	return &subscriptionWrapper{sub}, e
}

// subscriptionWrapper extends a pubsub.Subscription in order to wrap the Message type.
type subscriptionWrapper struct {
	*libp2p.Subscription
}

// Next wraps pubsub.Subscription.Next, implicitly adapting *pubsub.Message to the Message interface.
func (w subscriptionWrapper) Next(ctx context.Context) (Message, error) {
	return w.Subscription.Next(ctx)
}
