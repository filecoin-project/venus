package blocksub_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/clock"
	"github.com/filecoin-project/venus/internal/pkg/consensus"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/net/blocksub"
	th "github.com/filecoin-project/venus/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

func TestBlockTopicValidator(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	mbv := th.NewStubBlockValidator()
	tv := blocksub.NewBlockTopicValidator(mbv, nil)
	builder := chain.NewBuilder(t, address.Undef)
	pid1 := th.RequireIntPeerID(t, 1)

	goodBlk := builder.BuildOnBlock(nil, func(b *chain.BlockBuilder) {})
	badBlk := builder.BuildOnBlock(nil, func(b *chain.BlockBuilder) {
		b.IncHeight(1)
	})

	mbv.StubSyntaxValidationForBlock(badBlk, fmt.Errorf("invalid block"))

	validator := tv.Validator()

	network := "gfctest"
	assert.Equal(t, blocksub.Topic(network), tv.Topic(network))
	assert.True(t, validator(ctx, pid1, blkToPubSub(t, goodBlk)))
	assert.False(t, validator(ctx, pid1, blkToPubSub(t, badBlk)))
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
	mclock := clock.NewFake(now)
	// block time will be 1 second
	blocktime := time.Second * 1
	propDelay := 200 * time.Millisecond

	// setup a block validator and a topic validator
	chainClock := clock.NewChainClockFromClock(uint64(now.Unix()), blocktime, propDelay, mclock)
	bv := consensus.NewDefaultBlockValidator(chainClock, nil, nil)
	btv := blocksub.NewBlockTopicValidator(bv)

	// setup a floodsub instance on the host and register the topic validator
	network := "gfctest"
	fsub1, err := pubsub.NewFloodSub(ctx, host1, pubsub.WithMessageSigning(false))
	require.NoError(t, err)
	err = fsub1.RegisterTopicValidator(btv.Topic(network), btv.Validator(), btv.Opts()...)
	require.NoError(t, err)

	// subscribe to the block validator topic
	top1, err := fsub1.Join(btv.Topic(network))
	require.NoError(t, err)
	sub1, err := top1.Subscribe()
	require.NoError(t, err)

	// generate a miner address for blocks
	miner := types.NewForTestGetter()()

	mclock.Advance(blocktime) // enter epoch 1

	// create an invalid block
	invalidBlk := &block.Block{
		Height:          1,
		Timestamp:       uint64(now.Add(time.Second * 60).Unix()), // invalid timestamp, 60 seconds in future
		StateRoot:       enccid.NewCid(types.NewCidForTestGetter()()),
		Miner:           miner,
		Ticket:          block.Ticket{VRFProof: []byte{0}},
		BlockSig:        &crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: []byte{}},
		BLSAggregateSig: &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte{}},
	}
	// publish the invalid block
	payload := blocksub.Payload{
		Header:      *invalidBlk,
		BLSMsgCids:  nil,
		SECPMsgCids: nil,
	}
	payloadBytes, err := encoding.Encode(payload)
	require.NoError(t, err)
	err = top1.Publish(ctx, payloadBytes)
	assert.NoError(t, err)

	// see FIXME below (#3285)
	time.Sleep(time.Millisecond * 100)

	// create a valid block
	validTime := chainClock.StartTimeOfEpoch(abi.ChainEpoch(1))
	validBlk := &block.Block{
		Height:          1,
		Timestamp:       uint64(validTime.Unix()),
		StateRoot:       enccid.NewCid(types.NewCidForTestGetter()()),
		Miner:           miner,
		Ticket:          block.Ticket{VRFProof: []byte{0}},
		BlockSig:        &crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: []byte{}},
		BLSAggregateSig: &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte{}},
	}
	// publish the invalid block
	payload = blocksub.Payload{
		Header:      *validBlk,
		BLSMsgCids:  nil,
		SECPMsgCids: nil,
	}
	payloadBytes, err = encoding.Encode(payload)
	require.NoError(t, err)
	err = top1.Publish(ctx, payloadBytes)
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
	var receivedPayload blocksub.Payload
	err = encoding.Decode(received.GetData(), &receivedPayload)
	require.NoError(t, err)

	// assert this block is the valid one
	assert.Equal(t, validBlk.Cid().String(), receivedPayload.Header.Cid().String())
}

// convert a types.Block to a pubsub message
func blkToPubSub(t *testing.T, blk *block.Block) *pubsub.Message {
	payload := blocksub.Payload{
		Header:      *blk,
		BLSMsgCids:  nil,
		SECPMsgCids: nil,
	}
	data, err := encoding.Encode(&payload)
	require.NoError(t, err)
	return &pubsub.Message{
		Message: &pubsubpb.Message{
			Data: data,
		},
	}
}

// returns a pubsub message that will not decode to a types.Block
func nonBlkPubSubMsg() *pubsub.Message {
	pbm := &pubsubpb.Message{
		Data: []byte("meow"),
	}
	return &pubsub.Message{
		Message: pbm,
	}
}
