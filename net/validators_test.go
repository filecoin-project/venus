package net_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p-pubsub/pb"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/net"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestBlockTopicValidator(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	mbv := th.NewStubBlockValidator()
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
	assert.True(t, validator(ctx, pid1, blkToPubSub(goodBlk)))
	assert.False(t, validator(ctx, pid1, blkToPubSub(badBlk)))
	assert.False(t, validator(ctx, pid1, nonBlkPubSubMsg()))
}

func TestBlockPubSubValidation(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	// setup a mock network and generate a host
	mn := mocknet.New(ctx)
	host1, err := mn.GenPeer()
	require.NoError(t, err)

	// create a fake clock to trigger block validation failures
	now := time.Unix(1234567890, 0)
	mclock := th.NewFakeSystemClock(now)
	// block time will be 1 second
	blocktime := time.Second * 1

	// setup a block validator and a topic validator
	bv := consensus.NewDefaultBlockValidator(blocktime, mclock)
	btv := net.NewBlockTopicValidator(bv)

	// setup a floodsub instance on the host and register the topic validator
	fsub1, err := pubsub.NewFloodSub(ctx, host1, pubsub.WithMessageSigning(false))
	require.NoError(t, err)
	err = fsub1.RegisterTopicValidator(btv.Topic(), btv.Validator(), btv.Opts()...)
	require.NoError(t, err)

	// subscribe to the block validator topic
	sub1, err := fsub1.Subscribe(btv.Topic())
	require.NoError(t, err)

	// generate a miner address for blocks
	miner := address.NewForTestGetter()()

	// create an invalid block
	invalidBlk := &types.Block{
		Height:    1,
		Timestamp: types.Uint64(now.Add(time.Second * 60).Unix()), // invalid timestamp, 60 seconds in future
		StateRoot: types.NewCidForTestGetter()(),
		Miner:     miner,
		Tickets:   []types.Ticket{{VRFProof: []byte{0}}},
	}
	// publish the invalid block
	err = fsub1.Publish(btv.Topic(), invalidBlk.ToNode().RawData())
	assert.NoError(t, err)

	// see FIXME below (#3285)
	time.Sleep(time.Millisecond * 100)

	// create a valid block
	validBlk := &types.Block{
		Height:    1,
		Timestamp: types.Uint64(now.Unix()), // valid because it was publish "now".
		StateRoot: types.NewCidForTestGetter()(),
		Miner:     miner,
		Tickets:   []types.Ticket{{VRFProof: []byte{0}}},
	}
	// publish the invalid block
	err = fsub1.Publish(btv.Topic(), validBlk.ToNode().RawData())
	assert.NoError(t, err)

	// FIXME: #3285
	// Floodsub makes no guarantees on the order of messages, this means the block we
	// get here is nondeterministic. For now we do our best to let the invalid block propagate first
	// by sleeping (*wince*), but it could be the case that the valid block arrives first - meaning this
	// test could pass incorrectly since we don't know if the invalid block is in the channel and we
	// have no easy way of checking since Next blocks if the channel is empty. A solution here
	// could be to create a metrics registry in the block validator code and assert that it has seen
	// one invalid block and one valid block.
	// If this test ever flakes we know there is an issue with libp2p since the block validator has
	// a test and sine TestBlockTopicValidator tests the plumbing of this code.
	// This test should be reimplemented by starting an in-process node using something like GenNode
	// refer to #3285 for details.
	received, err := sub1.Next(ctx)
	assert.NoError(t, err, "Receieved an invalid block over pubsub, seee issue #3285 for help debugging")

	// decode the block from pubsub
	maybeBlk, err := types.DecodeBlock(received.GetData())
	require.NoError(t, err)

	// assert this block is the valid one
	assert.Equal(t, validBlk.Cid().String(), maybeBlk.Cid().String())
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
