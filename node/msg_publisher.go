package node

import (
	"context"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/pkg/errors"
)

// defaultMessagePublisher publishes messages to a pubsub topic and adds them to a message pool.
// This is wiring for message publication from the outbox.
type defaultMessagePublisher struct {
	network *pubsub.Publisher
	topic   string
	pool    *core.MessagePool
}

func newDefaultMessagePublisher(pubsub *pubsub.Publisher, topic string, pool *core.MessagePool) *defaultMessagePublisher {
	return &defaultMessagePublisher{pubsub, topic, pool}
}

func (p *defaultMessagePublisher) Publish(ctx context.Context, message *types.SignedMessage, height uint64) error {
	encoded, err := message.Marshal()
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}

	if _, err := p.pool.Add(ctx, message, height); err != nil {
		return errors.Wrap(err, "failed to add message to message pool")
	}

	if err = p.network.Publish(p.topic, encoded); err != nil {
		return errors.Wrap(err, "failed to publish message to network")
	}
	return nil
}
