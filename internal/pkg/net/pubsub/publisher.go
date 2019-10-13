package pubsub

import (
	"time"

	"github.com/libp2p/go-libp2p-pubsub"
)

// DefaultGossipsubHeartbeat is the default value of the gossipsub parameter.
// We store it here separately so that multiple tests can reference a static
// value instead of the mutable global var in the pubsub package.  This should
// be updated when go-libp2p-pubsub updates the variable.
const DefaultGossipsubHeartbeat = 100 * time.Millisecond

// Publisher publishes to pubsub topics
type Publisher struct {
	pubsub *pubsub.PubSub
}

// NewPublisher builds a new publisher
func NewPublisher(sub *pubsub.PubSub) *Publisher {
	return &Publisher{pubsub: sub}
}

// Publish publishes to a pubsub topic
func (s *Publisher) Publish(topic string, data []byte) error {
	return s.pubsub.Publish(topic, data)
}
