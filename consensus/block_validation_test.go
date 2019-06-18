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

// to block time converts a time to block timestamp
func tbt(t time.Time) types.Uint64 {
	return types.Uint64(t.Unix())
}

func TestBlockValidSemantic(t *testing.T) {
	tf.UnitTest(t)

	blockTime := consensus.DefaultBlockTime
	ts := time.Now()
	mclock := th.NewMockClock(ts)
	ctx := context.Background()

	validator := consensus.NewDefaultBlockValidator(blockTime, mclock)

	t.Run("reject block with same height as parents", func(t *testing.T) {
		c := &types.Block{Height: 1, Timestamp: tbt(ts)}
		p := &types.Block{Height: 1, Timestamp: tbt(ts)}
		parents, err := types.NewTipSet(p)
		require.NoError(t, err)

		err = validator.ValidateSemantic(ctx, c, &parents)
		assert.Equal(t, consensus.ErrInvalidHeight, err)
	})

	t.Run("reject block mined too soon after parent", func(t *testing.T) {
		c := &types.Block{Height: 2, Timestamp: tbt(ts)}
		p := &types.Block{Height: 1, Timestamp: tbt(ts)}
		parents, err := types.NewTipSet(p)
		require.NoError(t, err)

		err = validator.ValidateSemantic(ctx, c, &parents)
		assert.Equal(t, consensus.ErrTooSoon, err)
	})

	t.Run("reject block mined too soon after parent with one null block", func(t *testing.T) {
		c := &types.Block{Height: 3, Timestamp: tbt(ts)}
		p := &types.Block{Height: 1, Timestamp: tbt(ts)}
		parents, err := types.NewTipSet(p)
		require.NoError(t, err)

		err = validator.ValidateSemantic(ctx, c, &parents)
		assert.Equal(t, consensus.ErrTooSoon, err)
	})

	t.Run("accept block mined one block time after parent", func(t *testing.T) {
		c := &types.Block{Height: 2, Timestamp: tbt(ts.Add(blockTime))}
		p := &types.Block{Height: 1, Timestamp: tbt(ts)}
		parents, err := types.NewTipSet(p)
		require.NoError(t, err)

		err = validator.ValidateSemantic(ctx, c, &parents)
		assert.NoError(t, err)
	})

	t.Run("accept block mined one block time after parent with one null block", func(t *testing.T) {
		c := &types.Block{Height: 3, Timestamp: tbt(ts.Add(2 * blockTime))}
		p := &types.Block{Height: 1, Timestamp: tbt(ts)}
		parents, err := types.NewTipSet(p)
		require.NoError(t, err)

		err = validator.ValidateSemantic(ctx, c, &parents)
		assert.NoError(t, err)
	})

}

func TestBlockValidSyntax(t *testing.T) {
	tf.UnitTest(t)

	blockTime := consensus.DefaultBlockTime

	loc, _ := time.LoadLocation("America/New_York")
	ts := time.Date(2019, time.April, 1, 0, 0, 0, 0, loc)
	mclock := th.NewMockClock(ts)

	ctx := context.Background()

	validator := consensus.NewDefaultBlockValidator(blockTime, mclock)

	t.Run("reject block generated in future", func(t *testing.T) {
		blk := &types.Block{
			Timestamp: types.Uint64(ts.Add(time.Second * 1).Unix()),
		}
		assert.Error(t, validator.ValidateSyntax(ctx, blk))
	})

	t.Run("reject block with undef StateRoot", func(t *testing.T) {
		blk := &types.Block{
			StateRoot: cid.Undef,
		}
		assert.Error(t, validator.ValidateSyntax(ctx, blk))
	})

	t.Run("reject block with undef miner address", func(t *testing.T) {
		blk := &types.Block{
			Miner: address.Undef,
		}
		assert.Error(t, validator.ValidateSyntax(ctx, blk))
	})

	t.Run("reject block with empty ticket", func(t *testing.T) {
		blk := &types.Block{
			Ticket: []byte{},
		}
		assert.Error(t, validator.ValidateSyntax(ctx, blk))
	})
}
