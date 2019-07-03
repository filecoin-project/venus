package porcelain

import (
	"context"

	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/types"
)

// The subset of plumbing used by MessagePoolWait
type mpwPlumbing interface {
	MessagePoolPending() []*types.SignedMessage
	PubSubSubscribe(topic string) (pubsub.Subscription, error)
}

// MessagePoolWait waits until the message pool contains at least messageCount unmined messages.
func MessagePoolWait(ctx context.Context, plumbing mpwPlumbing, messageCount uint) ([]*types.SignedMessage, error) {
	pending := plumbing.MessagePoolPending()
	if len(pending) < int(messageCount) {
		subscription, err := plumbing.PubSubSubscribe(net.MessageTopic)
		defer subscription.Cancel()
		if err != nil {
			return nil, err
		}

		// Poll pending again after subscribing in case a message arrived since.
		pending = plumbing.MessagePoolPending()
		for len(pending) < int(messageCount) {
			_, err = subscription.Next(ctx)
			if err != nil {
				return nil, err
			}
			pending = plumbing.MessagePoolPending()
		}
	}

	return pending, nil
}
