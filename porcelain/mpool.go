package porcelain

import (
	"context"

	"gx/ipfs/QmepvmmYNM6q4RaUiwEikQFhgMFHXg2PLhx2E9iaRd3jmS/go-libp2p-pubsub"

	"github.com/filecoin-project/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/plumbing/ps"
	"github.com/filecoin-project/go-filecoin/types"
)

// The subset of plumbing used by MessagePoolWait
type mpwPlumbing interface {
	MessagePoolPending() []*types.SignedMessage
	PubSubSubscribe(topic string, opts ...pubsub.SubOpt) (ps.Subscription, error)
}

// MessagePoolWait waits until the message pool contains at least messageCount unmined messages.
func MessagePoolWait(ctx context.Context, plumbing mpwPlumbing, messageCount uint) ([]*types.SignedMessage, error) {
	pending := plumbing.MessagePoolPending()
	if len(pending) < int(messageCount) {
		subscription, err := plumbing.PubSubSubscribe(msg.Topic)
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
