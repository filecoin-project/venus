package node

import (
	"context"

	"gx/ipfs/QmSFihvoND3eDaAYRCeLgLPt62yCPgMZs1NSZmKFEtJQQw/go-libp2p-floodsub"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// BlocksTopic is the pubsub topic identifier on which new blocks are announced.
const BlocksTopic = "/fil/blocks"

// MessageTopic is the pubsub topic identifier on which new messages are announced.
const MessageTopic = "/fil/msgs"

// AddNewBlock processes a block on the local chain and publishes it to the network.
func (node *Node) AddNewBlock(ctx context.Context, b *types.Block) error {
	if _, err := node.ChainMgr.ProcessNewBlock(ctx, b); err != nil {
		return err
	}

	return node.PubSub.Publish(BlocksTopic, b.ToNode().RawData())
}

func (node *Node) handleBlockSubscription(ctx context.Context) {
	for {
		msg, err := node.BlockSub.Next(ctx)
		if err != nil {
			log.Errorf("BlockSub.Next(): %s", err)
			return
		}

		if err := node.processMessage(ctx, msg); err != nil {
			log.Errorf("processMessage(): %s", err)
		}
	}
}

func (node *Node) processMessage(ctx context.Context, msg *floodsub.Message) error {
	// ignore messages from ourself
	if msg.GetFrom() == node.Host.ID() {
		return nil
	}

	blk, err := types.DecodeBlock(msg.GetData())
	if err != nil {
		return errors.Wrap(err, "got bad block data")
	}

	res, err := node.ChainMgr.ProcessNewBlock(ctx, blk)
	if err != nil {
		return errors.Wrap(err, "processing block from network")
	}

	log.Infof("message processed: %s", res)
	return nil
}

// AddNewMessage adds a new message to the pool and publishes it to the network.
func (node *Node) AddNewMessage(ctx context.Context, msg *types.Message) error {
	if _, err := node.MsgPool.Add(msg); err != nil {
		return err
	}

	msgdata, err := msg.Marshal()
	if err != nil {
		return err
	}

	return node.PubSub.Publish(MessageTopic, msgdata)
}
