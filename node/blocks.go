package node

import (
	"context"

	"gx/ipfs/QmT5K5mHn2KUyCDBntKoojQJAJftNzutxzpYR33w8JdN6M/go-libp2p-floodsub"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

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

	// Put block in storage wired to an exchange so this node and other
	// nodes can fetch it.
	blkCid, err := node.OnlineStore.Put(ctx, b)
	if err != nil {
		return errors.Wrap(err, "could not add new block to online storage")
	}

	if err := node.Syncer.HandleNewBlocks(ctx, []*cid.Cid{blkCid}); err != nil {
		return err
	}

	// TODO: should this just be a cid? Right now receivers ask to fetch
	// the block over bitswap anyway.
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

	err = node.Syncer.HandleNewBlocks(ctx, []*cid.Cid{blk.Cid()})
	if err != nil {
		return errors.Wrap(err, "processing block from network")
	}
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
		return errors.Wrap(err, "failed to add message to the message pool")
	}

	msgdata, err := msg.Marshal()
	if err != nil {
		return errors.Wrap(err, "failed to marshal message")
	}

	return node.PubSub.Publish(MessageTopic, msgdata)
}
