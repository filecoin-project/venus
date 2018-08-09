package api_impl

import (
	"context"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

type NodeMpool struct {
	api *API
}

func NewNodeMpool(api *API) *NodeMpool {
	return &NodeMpool{api: api}
}

func (api *NodeMpool) View(ctx context.Context, messageCount uint) ([]*types.SignedMessage, error) {
	nd := api.api.node

	pending := nd.MsgPool.Pending()

	if len(pending) < int(messageCount) {
		subscription, err := nd.PubSub.Subscribe(node.MessageTopic)
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
