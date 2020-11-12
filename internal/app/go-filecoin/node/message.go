package node

import (
	"context"

	"github.com/filecoin-project/venus/internal/pkg/net/pubsub"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

func (node *Node) processMessage(ctx context.Context, pubSubMsg pubsub.Message) (err error) {
	sender := pubSubMsg.GetSender()

	// ignore messages from self
	if sender == node.Host().ID() {
		return nil
	}

	unmarshaled := &types.SignedMessage{}
	if err := unmarshaled.Unmarshal(pubSubMsg.GetData()); err != nil {
		return err
	}
	// TODO #3566 This is redundant with pubsub repeater validation.
	// We should do this in one call, maybe by waiting on pool add in repeater?
	err = node.Messaging.MsgSigVal.Validate(ctx, unmarshaled)
	if err != nil {
		return err
	}

	log.Debugf("Received new message %s from peer %s", unmarshaled, pubSubMsg.GetSender())

	_, err = node.Messaging.Inbox.Add(ctx, unmarshaled)
	return err
}
