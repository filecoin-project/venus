package node

import (
	"context"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/Qmc3BYVGtLs8y3p4uVpARWyo3Xk2oCBFF1AhYUVMPWgwUK/go-libp2p-pubsub"

	"github.com/filecoin-project/go-filecoin/types"
)

// MessageTopic is the pubsub topic identifier on which new messages are announced.
const MessageTopic = "/fil/msgs"

func (node *Node) processMessage(ctx context.Context, pubSubMsg *floodsub.Message) (err error) {
	ctx = log.Start(ctx, "Node.processMessage")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	unmarshaled := &types.SignedMessage{}
	if err := unmarshaled.Unmarshal(pubSubMsg.GetData()); err != nil {
		return err
	}
	log.SetTag(ctx, "message", unmarshaled)

	_, err = node.MsgPool.Add(unmarshaled)
	return err
}

// addNewMessage adds a new message to the pool, and publishes it to the network.
func (node *Node) addNewMessage(ctx context.Context, msg *types.SignedMessage) (err error) {
	ctx = log.Start(ctx, "Node.AddNewMessage")
	log.SetTag(ctx, "message", msg)
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	msgdata, err := msg.Marshal()
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}

	if _, err := node.MsgPool.Add(msg); err != nil {
		return errors.Wrap(err, "failed to add message to the message pool")
	}

	return node.PubSub.Publish(MessageTopic, msgdata)
}
