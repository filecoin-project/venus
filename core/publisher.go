package core

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/pkg/errors"
)

// DefaultMessagePublisher adds messages to a message pool and can publish them to its topic.
// This is wiring for message publication from the outbox.
type DefaultMessagePublisher struct {
	network networkPublisher
	topic   string
	pool    *MessagePool
}

type networkPublisher interface {
	Publish(topic string, data []byte) error
}

// NewDefaultMessagePublisher creates a new publisher.
func NewDefaultMessagePublisher(pubsub networkPublisher, topic string, pool *MessagePool) *DefaultMessagePublisher {
	return &DefaultMessagePublisher{pubsub, topic, pool}
}

// Publish marshals and publishes a message to the core message pool, and if bcast is true,
// broadcasts it to the network with the publisher's topic.
func (p *DefaultMessagePublisher) Publish(ctx context.Context, message *types.SignedMessage, height uint64, bcast bool) error {
	encoded, err := message.Marshal()
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}

	if _, err := p.pool.Add(ctx, message, height); err != nil {
		return errors.Wrap(err, "failed to add message to message pool")
	}

	if bcast {
		if err = p.network.Publish(p.topic, encoded); err != nil {
			return errors.Wrap(err, "failed to publish message to network")
		}
	}
	return nil
}
