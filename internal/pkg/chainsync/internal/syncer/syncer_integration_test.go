package syncer_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/internal/syncer"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/status"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// Syncer is capable of recovering from a fork reorg after the bsstore is loaded.
// This is a regression test to guard against the syncer assuming that the bsstore having all
// blocks from a tipset means the syncer has computed its state.
// Such a case happens when the bsstore has just loaded, but this tipset is not on its heaviest chain).
// See https://github.com/filecoin-project/go-filecoin/issues/1148#issuecomment-432008060
func TestLoadFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	// Set up in the standard way, but retain references to the repo and cbor stores.
	builder := chain.NewBuilder(t, address.Undef)
	genesis := builder.NewGenesis()
	genStateRoot, err := builder.GetTipSetStateRoot(genesis.Key())
	require.NoError(t, err)

	repo := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(repo.Datastore())
	cborStore := cborutil.NewIpldStore(bs)
	store := chain.NewStore(repo.ChainDatastore(), cborStore, chain.NewStatusReporter(), genesis.At(0).Cid())
	require.NoError(t, store.PutTipSetMetadata(ctx, &chain.TipSetMetadata{TipSetStateRoot: genStateRoot, TipSet: genesis, TipSetReceipts: types.EmptyReceiptsCID}))
	require.NoError(t, store.SetHead(ctx, genesis))

	// Note: the chain builder is passed as the fetcher, from which blocks may be requested, but
	// *not* as the bsstore, to which the syncer must ensure to put blocks.
	eval := &chain.FakeStateEvaluator{}
	sel := &chain.FakeChainSelector{}
	s, err := syncer.NewSyncer(eval, eval, sel, store, builder, builder, status.NewReporter(), clock.NewFake(time.Unix(1234567890, 0)), &noopFaultDetector{})
	require.NoError(t, err)
	require.NoError(t, s.InitStaged())

	base := builder.AppendManyOn(3, genesis)
	left := builder.AppendManyOn(4, base)
	right := builder.AppendManyOn(3, base)

	// Sync the two branches, which stores all blocks in the underlying stores.
	assert.NoError(t, s.HandleNewTipSet(ctx, block.NewChainInfo("", "", left.Key(), heightFromTip(t, left)), false))
	assert.NoError(t, s.HandleNewTipSet(ctx, block.NewChainInfo("", "", right.Key(), heightFromTip(t, right)), false))
	verifyHead(t, store, left)

	// The syncer/bsstore assume that the fetcher populates the underlying block bsstore such that
	// tipsets can be reconstructed. The chain builder used for testing doesn't do that, so do
	// it manually here.
	for _, tip := range []block.TipSet{left, right} {
		for itr := chain.IterAncestors(ctx, builder, tip); !itr.Complete(); require.NoError(t, itr.Next()) {
			for _, block := range itr.Value().ToSlice() {
				_, err := cborStore.Put(ctx, block)
				require.NoError(t, err)
			}
		}
	}

	// Load a new chain bsstore on the underlying data. It will only compute state for the
	// left (heavy) branch. It has a fetcher that can't provide blocks.
	newStore := chain.NewStore(repo.ChainDatastore(), cborStore, chain.NewStatusReporter(), genesis.At(0).Cid())
	require.NoError(t, newStore.Load(ctx))
	fakeFetcher := th.NewTestFetcher()
	offlineSyncer, err := syncer.NewSyncer(eval, eval, sel, newStore, builder, fakeFetcher, status.NewReporter(), clock.NewFake(time.Unix(1234567890, 0)), &noopFaultDetector{})
	require.NoError(t, err)
	require.NoError(t, offlineSyncer.InitStaged())

	assert.True(t, newStore.HasTipSetAndState(ctx, left.Key()))
	assert.False(t, newStore.HasTipSetAndState(ctx, right.Key()))

	// The newRight head extends right. The bsstore already has the individual blocks up to the point
	// `right`, but has not computed their state (because it's not the heavy branch).
	// Obtuse code organisation means that the syncer will
	// attempt to fetch `newRight` *and `right`* blocks from the network in the process of computing
	// the state sequence for them all. Yes, this is a bit silly - the `right` blocks are already local.
	// The test is guarding against a prior incorrect behaviour where the syncer would not attempt to
	// fetch the `right` blocks (because it already has them) but *also* would not compute their state.
	// We detect this by making the final `newRight` blocks fetchable, but not the `right` blocks, and
	// expect the syncer to fail due to that failed fetch.
	// This test would fail to work if the syncer could inspect the bsstore directly to avoid requesting
	// blocks already local, but also correctly recomputed the state.

	// Note that since the blocks are in the bsstore, and a real fetcher will consult the bsstore before
	// trying the network, this won't actually cause a network request. But it's really hard to follow.
	newRight := builder.AppendManyOn(1, right)
	fakeFetcher.AddSourceBlocks(newRight.ToSlice()...)

	// Test that the syncer can't sync a block chained from on the right (originally shorter) chain
	// without getting old blocks from network. i.e. the bsstore index has been trimmed
	// of non-heaviest chain blocks.

	err = offlineSyncer.HandleNewTipSet(ctx, block.NewChainInfo("", "", newRight.Key(), heightFromTip(t, newRight)), false)
	assert.Error(t, err)

	// The left chain is ok without any fetching though.
	assert.NoError(t, offlineSyncer.HandleNewTipSet(ctx, block.NewChainInfo("", "", left.Key(), heightFromTip(t, left)), false))
}

type noopFaultDetector struct{}

func (fd *noopFaultDetector) CheckBlock(_ *block.Block, _ block.TipSet) error {
	return nil
}
