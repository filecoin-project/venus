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

type floodSubProcessorFunc func(ctx context.Context, msg *floodsub.Message) error

func (node *Node) handleSubscription(ctx context.Context, f floodSubProcessorFunc, fname string, s *floodsub.Subscription, sname string) {
	for {
		pubSubMsg, err := s.Next(ctx)
		if err != nil {
			log.Errorf("%s.Next(): %s", sname, err)
			return
		}

		if err := f(ctx, pubSubMsg); err != nil {
			log.Errorf("%s(): %s", fname, err)
		}
	}
}

func (node *Node) processBlock(ctx context.Context, pubSubMsg *floodsub.Message) error {
	// ignore messages from ourself
	if pubSubMsg.GetFrom() == node.Host.ID() {
		return nil
	}

	blk, err := types.DecodeBlock(pubSubMsg.GetData())
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

func (node *Node) processMessage(ctx context.Context, pubSubMsg *floodsub.Message) error {
	unmarshaled := &types.Message{}
	if err := unmarshaled.Unmarshal(pubSubMsg.GetData()); err != nil {
		return err
	}

	_, err := node.MsgPool.Add(unmarshaled)
	return err
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
