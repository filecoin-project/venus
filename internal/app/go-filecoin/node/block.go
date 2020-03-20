package node

import (
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net/blocksub"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net/pubsub"
)

// AddNewBlock receives a newly mined block and stores, validates and propagates it to the network.
func (node *Node) AddNewBlock(ctx context.Context, o mining.Output) (err error) {
	b := o.Header
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

	// Publish blocksub message
	log.Debugf("publishing new block: %s", b.Cid().String())
	go func() {
		payload, err := blocksub.MakePayload(o.Header, o.BLSMessages, o.SECPMessages)
		if err != nil {
			log.Errorf("failed to create blocksub payload: %s", err)
		}
		err = node.syncer.BlockTopic.Publish(ctx, payload)
		if err != nil {
			log.Errorf("failed to publish on blocksub: %s", err)
		}
	}()

	log.Debugf("syncing new block: %s", b.Cid().String())
	ci := block.NewChainInfo(node.Host().ID(), node.Host().ID(), block.NewTipSetKey(blkCid), b.Height)
	return node.syncer.ChainSyncManager.BlockProposer().SendOwnBlock(ci)
}

func (node *Node) handleBlockSub(ctx context.Context, msg pubsub.Message) (err error) {
	sender := msg.GetSender()
	source := msg.GetSource()
	// ignore messages from self
	if sender == node.Host().ID() || source == node.Host().ID() {
		return nil
	}

	ctx, span := trace.StartSpan(ctx, "Node.handleBlockSub")
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	var payload blocksub.Payload
	err = encoding.Decode(msg.GetData(), &payload)
	if err != nil {
		return errors.Wrapf(err, "failed to decode blocksub payload from source: %s, sender: %s", source, sender)
	}

	header := &payload.Header
	span.AddAttributes(trace.StringAttribute("block", header.Cid().String()))
	log.Infof("Received new block %s from peer %s", header.Cid(), sender)
	log.Debugf("Received new block sender: %s source: %s, %s", sender, source, header)

	// The block we went to all that effort decoding is dropped on the floor!
	// Don't be too quick to change that, though: the syncer re-fetching the block
	// is currently critical to reliable validation.
	// See https://github.com/filecoin-project/go-filecoin/issues/2962
	// TODO Implement principled trusting of ChainInfo's
	// to address in #2674
	chainInfo := block.NewChainInfo(source, sender, block.NewTipSetKey(header.Cid()), header.Height)
	err = node.syncer.ChainSyncManager.BlockProposer().SendGossipBlock(chainInfo)
	if err != nil {
		return errors.Wrapf(err, "failed to notify syncer of new block, block: %s", header.Cid())
	}

	return nil
}
