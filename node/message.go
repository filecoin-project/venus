package node

import (
	"context"

	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/types"
)

func (node *Node) processMessage(ctx context.Context, pubSubMsg pubsub.Message) (err error) {
	ctx = log.Start(ctx, "Node.processMessage")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	unmarshaled := &types.SignedMessage{}
	if err := unmarshaled.Unmarshal(pubSubMsg.GetData()); err != nil {
		return err
	}
	log.SetTag(ctx, "message", unmarshaled)

	log.Debugf("Received new message from network: %s", unmarshaled)

	_, err = node.Inbox.Add(ctx, unmarshaled)
	return err
}
