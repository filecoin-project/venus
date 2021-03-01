package blocksub_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/types"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/net/blocksub"
	th "github.com/filecoin-project/venus/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
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
	assert.True(t, validator(ctx, pid1, blkToPubSub(t, goodBlk)) == pubsub.ValidationAccept)
	assert.False(t, validator(ctx, pid1, blkToPubSub(t, badBlk)) == pubsub.ValidationAccept)
	assert.False(t, validator(ctx, pid1, nonBlkPubSubMsg()) == pubsub.ValidationAccept)
}

// convert a types.BlockHeader to a pubsub message
func blkToPubSub(t *testing.T, blk *types.BlockHeader) *pubsub.Message {
	bm := types.BlockMsg{
		Header:        blk,
		BlsMessages:   nil,
		SecpkMessages: nil,
	}
	buf := new(bytes.Buffer)
	err := bm.MarshalCBOR(buf)
	require.NoError(t, err)

	return &pubsub.Message{
		Message: &pubsubpb.Message{
			Data: buf.Bytes(),
		},
	}
}

// returns a pubsub message that will not decode to a types.BlockHeader
func nonBlkPubSubMsg() *pubsub.Message {
	pbm := &pubsubpb.Message{
		Data: []byte("meow"),
	}
	return &pubsub.Message{
		Message: pbm,
	}
}
