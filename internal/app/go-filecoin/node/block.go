package node

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net/pubsub"
)

// AddNewBlock receives a newly mined block and stores, validates and propagates it to the network.
func (node *Node) AddNewBlock(ctx context.Context, b *block.Block) (err error) {
	ctx, span := trace.StartSpan(ctx, "Node.AddNewBlock")
	span.AddAttributes(trace.StringAttribute("block", b.Cid().String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	// Put block in storage wired to an exchange so this node and other
	// nodes can fetch it.
	log.Debugf("putting block in bitswap exchange: %s", b.Cid().String())
	blkCid, err := node.Blockstore.CborStore.Put(ctx, b)
	if err != nil {
		return errors.Wrap(err, "could not add new block to online storage")
	}

	log.Debugf("syncing new block: %s", b.Cid().String())
	go func() {
		err = node.syncer.BlockTopic.Publish(ctx, b.ToNode().RawData())
		if err != nil {
			log.Errorf("error publishing new block on block topic %s", err)
		}
	}()
	ci := block.NewChainInfo(node.Host().ID(), node.Host().ID(), block.NewTipSetKey(blkCid), uint64(b.Height))
	return node.syncer.ChainSyncManager.BlockProposer().SendOwnBlock(ci)
}

func (node *Node) processBlock(ctx context.Context, msg pubsub.Message) (err error) {
	sender := msg.GetSender()
	source := msg.GetSource()

	// ignore messages from self
	if sender == node.Host().ID() || source == node.Host().ID() {
		return nil
	}

	ctx, span := trace.StartSpan(ctx, "Node.processBlock")
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	blk, err := block.DecodeBlock(msg.GetData())
	if err != nil {
		return errors.Wrapf(err, "bad block data from source: %s, sender: %s", source, sender)
	}

	span.AddAttributes(trace.StringAttribute("block", blk.Cid().String()))

	log.Infof("Received new block %s from peer %s", blk.Cid(), sender)
	log.Debugf("Received new block sender: %s source: %s, %s", sender, source, blk)

	// The block we went to all that effort decoding is dropped on the floor!
	// Don't be too quick to change that, though: the syncer re-fetching the block
	// is currently critical to reliable validation.
	// See https://github.com/filecoin-project/go-filecoin/issues/2962
	// TODO Implement principled trusting of ChainInfo's
	// to address in #2674
	err = node.syncer.ChainSyncManager.BlockProposer().SendGossipBlock(block.NewChainInfo(source, sender, block.NewTipSetKey(blk.Cid()), uint64(blk.Height)))
	if err != nil {
		return errors.Wrapf(err, "failed to notify syncer of new block, block: %s", blk.Cid())
	}

	return nil
}
