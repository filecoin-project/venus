package node

import (
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/venus/internal/pkg/net/blocksub"
	"github.com/filecoin-project/venus/internal/pkg/net/pubsub"
)

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
	// See https://github.com/filecoin-project/venus/issues/2962
	// TODO Implement principled trusting of ChainInfo's
	// to address in #2674
	chainInfo := block.NewChainInfo(source, sender, block.NewTipSetKey(header.Cid()), header.Height)
	err = node.syncer.ChainSyncManager.BlockProposer().SendGossipBlock(chainInfo)
	if err != nil {
		return errors.Wrapf(err, "failed to notify syncer of new block, block: %s", header.Cid())
	}

	return nil
}
