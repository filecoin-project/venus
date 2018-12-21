package node

import (
	"context"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVRxA4J3UPQpw74dLrQ6NJkfysCA1H4GU28gVpXQt9zMU/go-libp2p-pubsub"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// BlockTopic is the pubsub topic identifier on which new blocks are announced.
const BlockTopic = "/fil/blocks"

// AddNewBlock receives a newly mined block and stores, validates and propagates it to the network.
func (node *Node) AddNewBlock(ctx context.Context, b *types.Block) (err error) {
	ctx = log.Start(ctx, "Node.AddNewBlock")
	log.SetTag(ctx, "block", b)
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	// Put block in storage wired to an exchange so this node and other
	// nodes can fetch it.
	log.Debugf("putting block in bitswap exchange: %s", b.Cid().String())
	blkCid, err := node.OnlineStore.Put(ctx, b)
	if err != nil {
		return errors.Wrap(err, "could not add new block to online storage")
	}

	log.Debugf("syncing new block: %s", b.Cid().String())
	if err := node.Syncer.HandleNewBlocks(ctx, []cid.Cid{blkCid}); err != nil {
		return err
	}

	// TODO: should this just be a cid? Right now receivers ask to fetch
	// the block over bitswap anyway.
	return node.PubSub.Publish(BlockTopic, b.ToNode().RawData())
}

func (node *Node) processBlock(ctx context.Context, pubSubMsg *pubsub.Message) (err error) {
	ctx = log.Start(ctx, "Node.processBlock")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	// ignore messages from ourself
	if pubSubMsg.GetFrom() == node.Host().ID() {
		return nil
	}

	blk, err := types.DecodeBlock(pubSubMsg.GetData())
	if err != nil {
		return errors.Wrap(err, "got bad block data")
	}
	log.SetTag(ctx, "block", blk)

	log.Debugf("Received new block from network: %s", blk)

	err = node.Syncer.HandleNewBlocks(ctx, []cid.Cid{blk.Cid()})
	if err != nil {
		return errors.Wrap(err, "processing block from network")
	}
	return nil
}
