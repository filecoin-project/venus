package node

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/net/pubsub"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func (node *Node) processMessage(ctx context.Context, pubSubMsg pubsub.Message) (err error) {
	unmarshaled := &types.SignedMessage{}
	if err := unmarshaled.Unmarshal(pubSubMsg.GetData()); err != nil {
		return err
	}

	log.Debugf("Received new message %s from peer %s", unmarshaled, pubSubMsg.GetSender())

	_, err = node.Messaging.Inbox.Add(ctx, unmarshaled)
	return err
}
