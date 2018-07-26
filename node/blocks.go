package node

import (
	"context"

	"gx/ipfs/QmSPD4WJu73TE4eJgzbZQTpmfyT5hsh3SEsZnpBAXpaBDA/go-libp2p-floodsub"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// BlocksTopic is the pubsub topic identifier on which new blocks are announced.
const BlocksTopic = "/fil/blocks"

// MessageTopic is the pubsub topic identifier on which new messages are announced.
const MessageTopic = "/fil/msgs"

// AddNewBlock processes a block on the local chain and publishes it to the network.
func (node *Node) AddNewBlock(ctx context.Context, b *types.Block) (err error) {
	ctx = log.Start(ctx, "Node.AddNewBlock")
	log.SetTag(ctx, "block", b)
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

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

func (node *Node) processBlock(ctx context.Context, pubSubMsg *floodsub.Message) (err error) {
	ctx = log.Start(ctx, "Node.processBlock")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	// ignore messages from ourself
	if pubSubMsg.GetFrom() == node.Host.ID() {
		return nil
	}

	blk, err := types.DecodeBlock(pubSubMsg.GetData())
	if err != nil {
		return errors.Wrap(err, "got bad block data")
	}
	log.SetTag(ctx, "block", blk)

	res, err := node.ChainMgr.ProcessNewBlock(ctx, blk)
	if err != nil {
		return errors.Wrap(err, "processing block from network")
	}

	log.Infof("message processed: %s", res)
	return nil
}

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

// AddNewMessage adds a new message to the pool, signs it with `node`s wallet,
// and publishes it to the network.
func (node *Node) AddNewMessage(ctx context.Context, msg *types.SignedMessage) (err error) {
	ctx = log.Start(ctx, "Node.AddNewMessage")
	log.SetTag(ctx, "message", msg)
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	if _, err := node.MsgPool.Add(msg); err != nil {
		return err
	}

	msgdata, err := msg.Marshal()
	if err != nil {
		return err
	}

	return node.PubSub.Publish(MessageTopic, msgdata)
}
