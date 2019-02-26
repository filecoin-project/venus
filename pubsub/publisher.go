package pubsub

import "gx/ipfs/QmepvmmYNM6q4RaUiwEikQFhgMFHXg2PLhx2E9iaRd3jmS/go-libp2p-pubsub"

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
