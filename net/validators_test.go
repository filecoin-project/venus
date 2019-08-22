package net_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/net"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestBlockTopicValidator(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	mbv := th.NewMockBlockValidator()
	tv := net.NewBlockTopicValidator(mbv, nil)
	builder := chain.NewBuilder(t, address.Undef)
	pid1 := th.RequireIntPeerID(t, 1)

	goodBlk := builder.BuildOnBlock(nil, func(b *chain.BlockBuilder) {})
	badBlk := builder.BuildOnBlock(nil, func(b *chain.BlockBuilder) {
		b.IncHeight(1)
	})

	mbv.StubSyntaxValidationForBlock(badBlk, fmt.Errorf("invalid block"))

	validator := tv.Validator()

	assert.Equal(t, net.BlockTopic, tv.Topic())
	assert.Nil(t, tv.Opts())
	assert.True(t, validator(ctx, pid1, blkToPubSub(goodBlk)))
	assert.False(t, validator(ctx, pid1, blkToPubSub(badBlk)))
	assert.False(t, validator(ctx, pid1, nonBlkPubSubMsg()))

}

// convert a types.Block to a pubsub message
func blkToPubSub(blk *types.Block) *pubsub.Message {
	pbm := &pubsub_pb.Message{
		Data: blk.ToNode().RawData(),
	}
	return &pubsub.Message{
		Message: pbm,
	}
}

// returns a pubsub message that will not decode to a types.Block
func nonBlkPubSubMsg() *pubsub.Message {
	pbm := &pubsub_pb.Message{
		Data: []byte("meow"),
	}
	return &pubsub.Message{
		Message: pbm,
	}
}
