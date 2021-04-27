package syncer_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/venus/pkg/chainsync/types"
	types2 "github.com/filecoin-project/venus/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/syncer"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/fork"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

// Syncer is capable of recovering from a fork reorg after the bsstore is loaded.
// This is a regression test to guard against the syncer assuming that the bsstore having all
// blocks from a tipset means the syncer has computed its state.
// Such a case happens when the bsstore has just loaded, but this tipset is not on its heaviest chain).
// See https://github.com/filecoin-project/venus/issues/1148#issuecomment-432008060
func TestLoadFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	// Set up in the standard way, but retain references to the repo and cbor stores.
	builder := chain.NewBuilder(t, address.Undef)
	genesis := builder.Genesis()

	// Note: the chain builder is passed as the fetcher, from which blocks may be requested, but
	// *not* as the bsstore, to which the syncer must ensure to put blocks.
	eval := &chain.FakeStateEvaluator{MessageStore: builder.Mstore()}
	sel := &chain.FakeChainSelector{}
	s, err := syncer.NewSyncer(eval, eval, sel, builder.Store(), builder.Mstore(), builder.BlockStore(), builder, clock.NewFake(time.Unix(1234567890, 0)), nil)
	require.NoError(t, err)

	base := builder.AppendManyOn(3, genesis)
	left := builder.AppendManyOn(4, base)
	right := builder.AppendManyOn(3, base)

	leftTarget := &types.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types2.NewChainInfo("", "", left),
	}
	rightTarget := &types.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types2.NewChainInfo("", "", right),
	}
	// Sync the two branches, which stores all blocks in the underlying stores.
	assert.NoError(t, s.HandleNewTipSet(ctx, leftTarget))
	assert.Error(t, s.HandleNewTipSet(ctx, rightTarget))
	verifyHead(t, builder.Store(), left)

	// The syncer/bsstore assume that the fetcher populates the underlying block bsstore such that
	// tipsets can be reconstructed. The chain builder used for testing doesn't do that, so do
	// it manually here.
	for _, tip := range []*types2.TipSet{left, right} {
		for itr := chain.IterAncestors(ctx, builder, tip); !itr.Complete(); require.NoError(t, itr.Next()) {
			for _, block := range itr.Value().ToSlice() {
				_, err := builder.Cstore().Put(ctx, block)
				require.NoError(t, err)
			}
		}
	}

	// Load a new chain bsstore on the underlying data. It will only compute state for the
	// left (heavy) branch. It has a fetcher that can't provide blocks.
	newStore := chain.NewStore(builder.Repo().ChainDatastore(), builder.Cstore(), builder.BlockStore(), config.DefaultForkUpgradeParam, genesis.At(0).Cid())
	newStore.SetCheckPoint(genesis.Key())
	require.NoError(t, newStore.Load(ctx))
	_, err = syncer.NewSyncer(eval,
		eval,
		sel,
		newStore,
		builder.Mstore(),
		builder.BlockStore(),
		builder,
		clock.NewFake(time.Unix(1234567890, 0)),
		fork.NewMockFork())
	require.NoError(t, err)

	assert.True(t, newStore.HasTipSetAndState(ctx, left))
	assert.False(t, newStore.HasTipSetAndState(ctx, right))
}
