package pubsub

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	libp2p "github.com/libp2p/go-libp2p-pubsub"
)

// Topic publishes and subscribes to a libp2p pubsub topic
type Topic struct {
	pubsubTopic *libp2p.Topic
}

// Message defines the common interface for go-filecoin message consumers.
// It's a subset of the go-libp2p-pubsub/pubsub.go Message type.
type Message interface {
	GetSource() peer.ID
	GetSender() peer.ID
	GetData() []byte
}

type message struct {
	inner *libp2p.Message
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

// NewTopic builds a new topic.
func NewTopic(topic *libp2p.Topic) *Topic {
	return &Topic{pubsubTopic: topic}
}

// Subscribe subscribes to a pubsub topic
func (t *Topic) Subscribe() (Subscription, error) {
	sub, err := t.pubsubTopic.Subscribe()
	return &subscriptionWrapper{sub}, err
}

// Publish publishes to a pubsub topic. It blocks until there is at least one
// peer on the mesh that can receive the publish.
func (t *Topic) Publish(ctx context.Context, data []byte) error {
	//	return t.pubsubTopic.Publish(ctx, data)
	return t.pubsubTopic.Publish(ctx, data, libp2p.WithReadiness(libp2p.MinTopicSize(1)))
}

// subscriptionWrapper extends a pubsub.Subscription in order to wrap the Message type.
type subscriptionWrapper struct {
	*libp2p.Subscription
}

// Next wraps pubsub.Subscription.Next, implicitly adapting *pubsub.Message to the Message interface.
func (w subscriptionWrapper) Next(ctx context.Context) (Message, error) {
	msg, err := w.Subscription.Next(ctx)
	if err != nil {
		return nil, err
	}
	return message{
		inner: msg,
	}, nil
}

func (m message) GetSender() peer.ID {
	return m.inner.ReceivedFrom
}

func (m message) GetSource() peer.ID {
	return m.inner.GetFrom()
}

func (m message) GetData() []byte {
	return m.inner.GetData()
}
