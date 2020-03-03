package consensus_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

func TestBlockValidSemantic(t *testing.T) {
	tf.UnitTest(t)

	blockTime := clock.DefaultEpochDuration
	ts := time.Unix(1234567890, 0)
	genTime := ts
	mclock := clock.NewChainClockFromClock(uint64(genTime.Unix()), blockTime, th.NewFakeClock(ts))
	ctx := context.Background()

	validator := consensus.NewDefaultBlockValidator(mclock)

	t.Run("reject block with same height as parents", func(t *testing.T) {
		// passes with valid height
		c := &block.Block{Height: 2, Timestamp: uint64(ts.Add(blockTime).Unix())}
		p := &block.Block{Height: 1, Timestamp: uint64(ts.Unix())}
		parents := consensus.RequireNewTipSet(require.New(t), p)
		require.NoError(t, validator.ValidateSemantic(ctx, c, parents))

		// invalidate parent by matching child height
		p = &block.Block{Height: 2, Timestamp: uint64(ts.Unix())}
		parents = consensus.RequireNewTipSet(require.New(t), p)

		err := validator.ValidateSemantic(ctx, c, parents)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid height")
	})
}

func TestMismatchedTime(t *testing.T) {
	tf.UnitTest(t)

	blockTime := clock.DefaultEpochDuration
	genTime := time.Unix(1234567890, 1234567890%int64(time.Second))
	fc := th.NewFakeClock(genTime)
	mclock := clock.NewChainClockFromClock(uint64(genTime.Unix()), blockTime, fc)
	validator := consensus.NewDefaultBlockValidator(mclock)

	fc.Advance(blockTime)

	// Passes with correct timestamp
	c := &block.Block{Height: 1, Timestamp: uint64(fc.Now().Unix())}
	require.NoError(t, validator.TimeMatchesEpoch(c))

	// fails with invalid timestamp
	c = &block.Block{Height: 1, Timestamp: uint64(genTime.Unix())}
	err := validator.TimeMatchesEpoch(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "wrong epoch")
}

func TestFutureEpoch(t *testing.T) {
	tf.UnitTest(t)

	blockTime := clock.DefaultEpochDuration
	genTime := time.Unix(1234567890, 1234567890%int64(time.Second))
	fc := th.NewFakeClock(genTime)
	mclock := clock.NewChainClockFromClock(uint64(genTime.Unix()), blockTime, fc)
	validator := consensus.NewDefaultBlockValidator(mclock)

	// Fails in future epoch
	c := &block.Block{Height: 1, Timestamp: uint64(genTime.Add(blockTime).Unix())}
	err := validator.NotFutureBlock(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "future epoch")
}

func TestBlockValidSyntax(t *testing.T) {
	tf.UnitTest(t)

	blockTime := clock.DefaultEpochDuration
	ts := time.Unix(1234567890, 0)
	mclock := th.NewFakeClock(ts)
	chainClock := clock.NewChainClockFromClock(uint64(ts.Unix()), blockTime, mclock)
	ctx := context.Background()
	mclock.Advance(blockTime)

	validator := consensus.NewDefaultBlockValidator(chainClock)

	validTs := uint64(mclock.Now().Unix())
	validSt := e.NewCid(types.NewCidForTestGetter()())
	validAd := vmaddr.NewForTestGetter()()
	validTi := block.Ticket{VRFProof: []byte{1}}
	validCandidate := block.NewEPoStCandidate(1, []byte{1}, 1)
	validPoStInfo := block.NewEPoStInfo(consensus.MakeFakePoStsForTest(), []byte{1}, validCandidate)
	// create a valid block
	blk := &block.Block{
		Timestamp: validTs,
		StateRoot: validSt,
		Miner:     validAd,
		Ticket:    validTi,
		Height:    1,

		EPoStInfo: validPoStInfo,
	}
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// below we will invalidate each part of the block, assert that it fails
	// validation, then revalidate the block

	// invalidate timestamp
	blk.Timestamp = uint64(ts.Add(time.Duration(3) * blockTime).Unix())
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.Timestamp = validTs
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// invalidate stateroot
	blk.StateRoot = e.NewCid(cid.Undef)
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.StateRoot = validSt
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// invalidate miner address
	blk.Miner = address.Undef
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.Miner = validAd
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

	// invalidate ticket
	blk.Ticket = block.Ticket{}
	require.Error(t, validator.ValidateSyntax(ctx, blk))
	blk.Ticket = validTi
	require.NoError(t, validator.ValidateSyntax(ctx, blk))

}
