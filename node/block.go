package node

import (
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/types"
)

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

	// TODO Implement principled trusting of ChainInfo's
	// to address in #2674
	trusted := true
	if err := node.Syncer.HandleNewTipSet(ctx, types.NewChainInfo(node.Host().ID(), types.NewTipSetKey(blkCid), uint64(b.Height)), trusted); err != nil {
		return err
	}

	return node.PorcelainAPI.PubSubPublish(net.BlockTopic, b.ToNode().RawData())
}

func (node *Node) processBlock(ctx context.Context, pubSubMsg pubsub.Message) (err error) {
	from := pubSubMsg.GetFrom()
	// ignore messages from ourself
	if from == node.Host().ID() {
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

	// The block we went to all that effort decoding is dropped on the floor!
	// Don't be too quick to change that, though: the syncer re-fetching the block
	// is currently critical to reliable validation.
	// See https://github.com/filecoin-project/go-filecoin/issues/2962
	// TODO Implement principled trusting of ChainInfo's
	// to address in #2674
	trusted := true
	err = node.Syncer.HandleNewTipSet(ctx, types.NewChainInfo(from, types.NewTipSetKey(blk.Cid()), uint64(blk.Height)), trusted)
	if err != nil {
		return errors.Wrap(err, "processing block from network")
	}

	return nil
}
