package node

import (
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/types"
)

// BlockTopic is the pubsub topic identifier on which new blocks are announced.
const BlockTopic = "/fil/blocks"

// AddNewBlock receives a newly mined block and stores, validates and propagates it to the network.
func (node *Node) AddNewBlock(ctx context.Context, b *types.Block) (err error) {
	ctx, span := trace.StartSpan(ctx, "Node.AddNewBlock")
	span.AddAttributes(trace.StringAttribute("block", b.Cid().String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	// Put block in storage wired to an exchange so this node and other
	// nodes can fetch it.
	log.Debugf("putting block in bitswap exchange: %s", b.Cid().String())
	blkCid, err := node.cborStore.Put(ctx, b)
	if err != nil {
		return errors.Wrap(err, "could not add new block to online storage")
	}

	log.Debugf("syncing new block: %s", b.Cid().String())
	if err := node.Syncer.HandleNewTipset(ctx, types.NewSortedCidSet(blkCid)); err != nil {
		return err
	}

	// TODO: should this just be a cid? Right now receivers ask to fetch
	// the block over bitswap anyway.
	return node.PorcelainAPI.PubSubPublish(BlockTopic, b.ToNode().RawData())
}

func (node *Node) processBlock(ctx context.Context, pubSubMsg pubsub.Message) (err error) {
	// ignore messages from ourself
	if pubSubMsg.GetFrom() == node.Host().ID() {
		return nil
	}

	ctx, span := trace.StartSpan(ctx, "Node.processBlock")
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	blk, err := types.DecodeBlock(pubSubMsg.GetData())
	if err != nil {
		return errors.Wrap(err, "got bad block data")
	}
	span.AddAttributes(trace.StringAttribute("block", blk.Cid().String()))

	log.Infof("Received new block from network cid: %s", blk.Cid().String())
	log.Debugf("Received new block from network: %s", blk)

	err = node.Syncer.HandleNewTipset(ctx, types.NewSortedCidSet(blk.Cid()))
	if err != nil {
		return errors.Wrap(err, "processing block from network")
	}

	return nil
}
