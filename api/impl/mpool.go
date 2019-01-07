package impl

import (
	"context"

	"github.com/filecoin-project/go-filecoin/api2/impl/msg"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodeMpool struct {
	api *nodeAPI
}

func newNodeMpool(api *nodeAPI) *nodeMpool {
	return &nodeMpool{api: api}
}

func (api *nodeMpool) View(ctx context.Context, messageCount uint) ([]*types.SignedMessage, error) {
	nd := api.api.node

	pending := nd.MsgPool.Pending()
	if len(pending) < int(messageCount) {
		subscription, err := nd.PubSub.Subscribe(msg.Topic)
		if err != nil {
			return nil, err
		}

		for len(pending) < int(messageCount) {
			_, err = subscription.Next(ctx)
			if err != nil {
				return nil, err
			}
			pending = nd.MsgPool.Pending()
		}
	}

	return pending, nil
}
