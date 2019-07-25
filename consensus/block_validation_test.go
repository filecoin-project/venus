package consensus_test

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestBlockValidSemantic(t *testing.T) {
	tf.UnitTest(t)

	blockTime := consensus.DefaultBlockTime
	ts := time.Unix(1234567890, 0)
	mclock := th.NewFakeSystemClock(ts)
	ctx := context.Background()

	validator := consensus.NewDefaultBlockValidator(blockTime, mclock)

	t.Run("reject block with same height as parents", func(t *testing.T) {
		// passes with valid height
		c := &types.Block{Height: 2, Timestamp: types.Uint64(ts.Add(blockTime).Unix())}
		p := &types.Block{Height: 1, Timestamp: types.Uint64(ts.Unix())}
		parents := consensus.RequireNewTipSet(require.New(t), p)
		require.NoError(t, validator.ValidateSemantic(ctx, c, &parents))

		// invalidate parent by matching child height
		p = &types.Block{Height: 2, Timestamp: types.Uint64(ts.Unix())}
		parents = consensus.RequireNewTipSet(require.New(t), p)

		err := validator.ValidateSemantic(ctx, c, &parents)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid height")

	})

	t.Run("reject block mined too soon after parent", func(t *testing.T) {
		// Passes with correct timestamp
		c := &types.Block{Height: 2, Timestamp: types.Uint64(ts.Add(blockTime).Unix())}
		p := &types.Block{Height: 1, Timestamp: types.Uint64(ts.Unix())}
		parents := consensus.RequireNewTipSet(require.New(t), p)
		require.NoError(t, validator.ValidateSemantic(ctx, c, &parents))

		// fails with invalid timestamp
		c = &types.Block{Height: 2, Timestamp: types.Uint64(ts.Unix())}
		err := validator.ValidateSemantic(ctx, c, &parents)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too far")

	})

	t.Run("reject block mined too soon after parent with one null block", func(t *testing.T) {
		// Passes with correct timestamp
		c := &types.Block{Height: 3, Timestamp: types.Uint64(ts.Add(2 * blockTime).Unix())}
		p := &types.Block{Height: 1, Timestamp: types.Uint64(ts.Unix())}
		parents := consensus.RequireNewTipSet(require.New(t), p)
		err := validator.ValidateSemantic(ctx, c, &parents)
		require.NoError(t, err)

		// fail when nul block calc is off by one blocktime
		c = &types.Block{Height: 3, Timestamp: types.Uint64(ts.Add(blockTime).Unix())}
		err = validator.ValidateSemantic(ctx, c, &parents)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too far")

		// fail with same timestamp as parent
		c = &types.Block{Height: 3, Timestamp: types.Uint64(ts.Unix())}
		err = validator.ValidateSemantic(ctx, c, &parents)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too far")

	})
}

func TestBlockValidSyntax(t *testing.T) {
	tf.UnitTest(t)

	blockTime := consensus.DefaultBlockTime
	ts := time.Unix(1234567890, 0)
	mclock := th.NewFakeSystemClock(ts)

	ctx := context.Background()

	validator := consensus.NewDefaultBlockValidator(blockTime, mclock)

	validTs := types.Uint64(ts.Unix())
	validSt := types.NewCidForTestGetter()()
	validAd := address.NewForTestGetter()()
	validTi := []byte{1}
	// create a valid block
	blk := &types.Block{
		Timestamp: validTs,
		StateRoot: validSt,
		Miner:     validAd,
		Ticket:    validTi,
		Height:    1,
	}
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// below we will invalidate each part of the block, assert that it fails
	// validation, then revalidate the block

	// invalidate timestamp
	blk.Timestamp = types.Uint64(ts.Add(time.Second).Unix())
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.Timestamp = validTs
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// invalidate statetooy
	blk.StateRoot = cid.Undef
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.StateRoot = validSt
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// invalidate miner address
	blk.Miner = address.Undef
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.Miner = validAd
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// invalidate ticket
	blk.Ticket = []byte{}
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.Ticket = validTi
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

}
