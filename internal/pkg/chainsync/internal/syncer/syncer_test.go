package syncer_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/internal/syncer"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/status"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func heightFromTip(t *testing.T, tip block.TipSet) uint64 {
	h, err := tip.Height()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestOneBlock(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	t1 := builder.AppendOn(genesis, 1)
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t1.Key(), heightFromTip(t, t1)), false))

	verifyTip(t, store, t1, t1.At(0).StateRoot)
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, t1)
}

func TestMultiBlockTip(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	tip := builder.AppendOn(genesis, 2)
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", tip.Key(), heightFromTip(t, tip)), false))

	verifyTip(t, store, tip, builder.StateForKey(tip.Key()))
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, tip)
}

func TestTipSetIncremental(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	t1 := builder.AppendOn(genesis, 1)

	t2 := builder.AppendOn(genesis, 1)

	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t1.Key(), heightFromTip(t, t1)), false))

	verifyTip(t, store, t1, builder.StateForKey(t1.Key()))
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, t1)

	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t2.Key(), heightFromTip(t, t2)), false))
	_, err := store.GetTipSet(t2.Key())
	require.Error(t, err)

	merged := th.RequireNewTipSet(t, t1.At(0), t2.At(0))
	verifyTip(t, store, merged, builder.StateForKey(merged.Key()))
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, merged)
}

func TestChainIncremental(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	t1 := builder.AppendOn(genesis, 2)

	t2 := builder.AppendOn(t1, 3)

	t3 := builder.AppendOn(t2, 1)

	t4 := builder.AppendOn(t3, 2)

	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t1.Key(), heightFromTip(t, t1)), false))
	verifyTip(t, store, t1, builder.StateForKey(t1.Key()))
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, t1)

	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t2.Key(), heightFromTip(t, t2)), false))
	verifyTip(t, store, t2, builder.StateForKey(t2.Key()))
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, t2)

	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t3.Key(), heightFromTip(t, t3)), false))
	verifyTip(t, store, t3, builder.StateForKey(t3.Key()))
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, t3)

	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t4.Key(), heightFromTip(t, t4)), false))
	verifyTip(t, store, t4, builder.StateForKey(t4.Key()))
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, t4)
}

func TestChainJump(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	t1 := builder.AppendOn(genesis, 2)
	t2 := builder.AppendOn(t1, 3)
	t3 := builder.AppendOn(t2, 1)
	t4 := builder.AppendOn(t3, 2)

	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t4.Key(), heightFromTip(t, t4)), false))
	verifyTip(t, store, t1, builder.StateForKey(t1.Key()))
	verifyTip(t, store, t2, builder.StateForKey(t2.Key()))
	verifyTip(t, store, t3, builder.StateForKey(t3.Key()))
	verifyTip(t, store, t4, builder.StateForKey(t4.Key()))
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, t4)
}

func TestIgnoreLightFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	forkbase := builder.AppendOn(genesis, 1)
	forkHead := builder.AppendOn(forkbase, 1)

	t1 := builder.AppendOn(forkbase, 1)
	t2 := builder.AppendOn(t1, 1)
	t3 := builder.AppendOn(t2, 1)
	t4 := builder.AppendOn(t3, 1)

	// Sync heaviest branch first.
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t4.Key(), heightFromTip(t, t4)), false))
	verifyTip(t, store, t4, builder.StateForKey(t4.Key()))
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, t4)

	// Lighter fork is processed but not change head.
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", forkHead.Key(), heightFromTip(t, forkHead)), false))
	_, err := store.GetTipSet(forkHead.Key())
	require.Error(t, err)
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, t4)
}

func TestAcceptHeavierFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	forkbase := builder.AppendOn(genesis, 1)

	main1 := builder.AppendOn(forkbase, 1)
	main2 := builder.AppendOn(main1, 1)
	main3 := builder.AppendOn(main2, 1)
	main4 := builder.AppendOn(main3, 1)

	// Fork is heavier with more blocks, despite shorter (with default fake weighing function
	// from FakeStateEvaluator).
	fork1 := builder.AppendOn(forkbase, 3)
	fork2 := builder.AppendOn(fork1, 1)
	fork3 := builder.AppendOn(fork2, 1)

	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", main4.Key(), heightFromTip(t, main4)), false))
	verifyTip(t, store, main4, builder.StateForKey(main4.Key()))
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, main4)

	// Heavier fork updates head3
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", fork3.Key(), heightFromTip(t, fork3)), false))
	verifyTip(t, store, fork1, builder.StateForKey(fork1.Key()))
	verifyTip(t, store, fork2, builder.StateForKey(fork2.Key()))
	verifyTip(t, store, fork3, builder.StateForKey(fork3.Key()))
	require.NoError(t, syncer.SetStagedHead(ctx))
	verifyHead(t, store, fork3)
}

func TestRejectFinalityFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, s := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	head := builder.AppendManyOn(consensus.FinalityEpochs+2, genesis)
	assert.NoError(t, s.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", head.Key(), heightFromTip(t, head)), false))

	// Differentiate fork for a new chain.  Fork has FinalityEpochs + 1
	// blocks on top of genesis so forkFinalityBase is more than FinalityEpochs
	// behind head
	forkFinalityBase := builder.BuildOneOn(genesis, func(bb *chain.BlockBuilder) {
		bb.SetTicket([]byte{0xbe})
	})
	forkFinalityHead := builder.AppendManyOn(consensus.FinalityEpochs, forkFinalityBase)
	assert.Error(t, s.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", forkFinalityHead.Key(), heightFromTip(t, forkFinalityHead)), false))
}

func TestNoUncessesaryFetch(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, s := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	head := builder.AppendManyOn(4, genesis)
	assert.NoError(t, s.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", head.Key(), heightFromTip(t, head)), false))

	// A new syncer unable to fetch blocks from the network can handle a tipset that's already
	// in the store and linked to genesis.
	emptyFetcher := chain.NewBuilder(t, address.Undef)
	newSyncer, err := syncer.NewSyncer(&chain.FakeStateEvaluator{}, &chain.FakeStateEvaluator{}, &chain.FakeChainSelector{}, store, builder, emptyFetcher, status.NewReporter(), th.NewFakeClock(time.Unix(1234567890, 0)), &noopFaultDetector{})
	require.NoError(t, err)
	require.NoError(t, newSyncer.InitStaged())
	assert.NoError(t, newSyncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", head.Key(), heightFromTip(t, head)), false))
}

// Syncer must track state of subsets of parent tipsets tracked in the store
// when they are the ancestor in a chain.  This is in order to maintain the
// invariant that the aggregate state of the  parents of the base of a collected chain
// is kept in the store.  This invariant allows chains built on subsets of
// tracked tipsets to be handled correctly.
// This test tests that the syncer stores the state of such a base tipset of a collected chain,
// i.e. a subset of an existing tipset in the store.
//
// Ex: {A1, A2} -> {B1, B2, B3} in store to start
// {B1, B2} -> {C1, C2} chain 1 input to syncer
// C1 -> D1 chain 2 input to syncer
//
// The last operation will fail if the state of subset {B1, B2} is not
// kept in the store because syncing C1 requires retrieving parent state.
func TestSubsetParent(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, s := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	// Set up chain with {A1, A2} -> {B1, B2, B3}
	tipA1A2 := builder.AppendOn(genesis, 2)
	tipB1B2B3 := builder.AppendOn(tipA1A2, 3)
	require.NoError(t, s.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", tipB1B2B3.Key(), heightFromTip(t, tipB1B2B3)), false))

	// Sync one tipset with a parent equal to a subset of an existing
	// tipset in the store: {B1, B2} -> {C1, C2}
	tipB1B2 := th.RequireNewTipSet(t, tipB1B2B3.At(0), tipB1B2B3.At(1))
	tipC1C2 := builder.AppendOn(tipB1B2, 2)

	assert.NoError(t, s.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", tipC1C2.Key(), heightFromTip(t, tipC1C2)), false))

	// Sync another tipset with a parent equal to a subset of the tipset
	// just synced: C1 -> D1
	tipC1 := th.RequireNewTipSet(t, tipC1C2.At(0))
	tipD1OnC1 := builder.AppendOn(tipC1, 1)
	assert.NoError(t, s.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", tipD1OnC1.Key(), heightFromTip(t, tipD1OnC1)), false))

	// A full parent also works fine: {C1, C2} -> D1
	tipD1OnC1C2 := builder.AppendOn(tipC1C2, 1)
	assert.NoError(t, s.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", tipD1OnC1C2.Key(), heightFromTip(t, tipD1OnC1C2)), false))
}

// Check that the syncer correctly adds widened chain ancestors to the store.
func TestWidenChainAncestor(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	link1 := builder.AppendOn(genesis, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.AppendOn(link3, 2)

	// Build another block with parents link1, but not included in link2.
	link2Alt := builder.AppendOn(link1, 1)
	// Build a tipset containing one block from link2, plus this new sibling.
	link2UnionSubset := th.RequireNewTipSet(t, link2.At(0), link2Alt.At(0))

	// Sync the subset of link2 first
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", link2UnionSubset.Key(), heightFromTip(t, link2UnionSubset)), false))
	verifyTip(t, store, link2UnionSubset, builder.StateForKey(link2UnionSubset.Key()))
	verifyHead(t, store, link2UnionSubset)

	// Sync chain with head at link4
	require.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", link4.Key(), heightFromTip(t, link4)), false))
	verifyTip(t, store, link4, builder.StateForKey(link4.Key()))
	verifyHead(t, store, link4)

	// Check that the widened tipset (link2UnionSubset U link2) is tracked
	link2Union := th.RequireNewTipSet(t, link2.At(0), link2.At(1), link2.At(2), link2Alt.At(0))
	verifyTip(t, store, link2Union, builder.StateForKey(link2Union.Key()))
}

// Syncer finds a heaviest tipset by combining blocks from the ancestors of a
// chain and blocks already in the store.
//
// A guide to this test -- the point is that sometimes when merging chains the syncer
// will find a new heaviest tipset that is not the head of either chain.  The syncer
// should correctly set this tipset as the head.
//
// From above we have the test-chain:
// genesis -> (link1blk1, link1blk2) -> (link2blk1, link2blk2, link2blk3) -> link3blk1 -> (link4blk1, link4blk2)
//
// Now we introduce a disjoint fork on top of link1
// genesis -> (link1blk1, link1blk2) -> (forklink2blk1, forklink2blk2, forklink2blk3, forklink3blk4) -> forklink3blk1
//
// When all blocks contribute equally to weight:
// So, the weight of the  head of the test chain =
//   W(link1) + 3 + 1 + 2 = W(link1) + 6 = 8
// and the weight of the head of the fork chain =
//   W(link1) + 4 + 1 = W(link1) + 5 = 7
// and the weight of the union of link2 of both branches (a valid tipset) is
//   W(link1) + 7 = 9
//
// Therefore the syncer should set the head of the store to the union of the links..
func TestHeaviestIsWidenedAncestor(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	link1 := builder.AppendOn(genesis, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.AppendOn(link3, 2)

	forkLink2 := builder.AppendOn(link1, 4)
	forkLink3 := builder.AppendOn(forkLink2, 1)

	// Sync main chain
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", link4.Key(), heightFromTip(t, link4)), false))

	// Sync fork chain
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", forkLink3.Key(), heightFromTip(t, forkLink3)), false))

	// Assert that widened chain is the new head
	wideBlocks := link2.ToSlice()
	wideBlocks = append(wideBlocks, forkLink2.ToSlice()...)
	wideTs := th.RequireNewTipSet(t, wideBlocks...)

	verifyTip(t, store, wideTs, builder.ComputeState(wideTs))
	verifyHead(t, store, wideTs)
}

func TestBlocksNotATipSetRejected(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	b1 := builder.AppendBlockOn(genesis)
	b2 := builder.AppendBlockOnBlocks(b1)

	badKey := block.NewTipSetKey(b1.Cid(), b2.Cid())
	err := syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", badKey, uint64(b1.Height)), false)
	assert.Error(t, err)

	_, err = store.GetTipSet(badKey)
	assert.Error(t, err) // Not present
}

func TestBlockNotLinkedRejected(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	// Set up a parallel builder from which the syncer cannot fetch.
	// The two builders are expected to produce exactly the same blocks from the same sequence
	// of calls.
	shadowBuilder := chain.NewBuilder(t, address.Undef)
	gen2 := th.RequireNewTipSet(t, shadowBuilder.AppendBlockOnBlocks())
	require.True(t, genesis.Equals(gen2))

	// The syncer fails to fetch this block so cannot sync it.
	b1 := shadowBuilder.AppendOn(genesis, 1)
	assert.Error(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", b1.Key(), heightFromTip(t, b1)), false))

	// Make the same block available from the syncer's builder
	builder.AppendBlockOn(genesis)
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", b1.Key(), heightFromTip(t, b1)), false))
}

type poisonValidator struct {
	headerFailureTS uint64
	fullFailureTS   uint64
}

func newPoisonValidator(t *testing.T, headerFailure, fullFailure uint64) *poisonValidator {
	return &poisonValidator{headerFailureTS: headerFailure, fullFailureTS: fullFailure}
}

func (pv *poisonValidator) RunStateTransition(_ context.Context, ts block.TipSet, _ [][]*types.UnsignedMessage, _ [][]*types.SignedMessage, _ []block.TipSet, _ uint64, _ cid.Cid, _ cid.Cid) (cid.Cid, []*types.MessageReceipt, error) {
	stamp := ts.At(0).Timestamp
	if pv.fullFailureTS == uint64(stamp) {
		return cid.Undef, nil, errors.New("run state transition fails on poison timestamp")
	}
	return cid.Undef, nil, nil
}

func (pv *poisonValidator) ValidateSemantic(_ context.Context, header *block.Block, _ block.TipSet) error {
	if pv.headerFailureTS == uint64(header.Timestamp) {
		return errors.New("val semantic fails on poison timestamp")
	}
	return nil
}

func TestSemanticallyBadTipSetFails(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	eval := newPoisonValidator(t, 98, 99)
	builder, store, syncer := setupWithValidator(ctx, t, eval, eval)
	genesis := builder.RequireTipSet(store.GetHead())

	// Build a chain with messages that will fail semantic header validation
	kis := types.MustGenerateKeyInfo(1, 42)
	mm := types.NewMessageMaker(t, kis)
	alice := mm.Addresses()[0]
	m1 := mm.NewSignedMessage(alice, 0)
	m2 := mm.NewSignedMessage(alice, 1)
	m3 := mm.NewSignedMessage(alice, 3)

	link1 := builder.BuildOneOn(genesis, func(bb *chain.BlockBuilder) {
		bb.AddMessages(
			[]*types.SignedMessage{m1, m2, m3},
			[]*types.UnsignedMessage{},
		)
		bb.SetTimestamp(98) // poison header val
	})

	// Set up a fresh builder without any of this data
	err := syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", link1.Key(), heightFromTip(t, link1)), false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "val semantic fails")
}

func TestSyncerStatus(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	// verify default status
	s0 := syncer.Status()
	assert.Equal(t, int64(0), s0.SyncingStarted)
	assert.Equal(t, block.UndefTipSet.Key(), s0.SyncingHead)
	assert.Equal(t, uint64(0), s0.SyncingHeight)
	assert.Equal(t, false, s0.SyncingTrusted)
	assert.Equal(t, true, s0.SyncingComplete)
	assert.Equal(t, true, s0.SyncingFetchComplete)
	assert.Equal(t, block.UndefTipSet.Key(), s0.FetchingHead)
	assert.Equal(t, uint64(0), s0.FetchingHeight)

	// initial sync and status check
	t1 := builder.AppendOn(genesis, 1)
	require.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t1.Key(), heightFromTip(t, t1)), false))
	s1 := syncer.Status()
	assert.Equal(t, t1.Key(), s1.FetchingHead)
	assert.Equal(t, uint64(1), s1.FetchingHeight)

	assert.Equal(t, true, s1.SyncingFetchComplete)
	assert.Equal(t, true, s1.SyncingComplete)

	// advance the chain head, ensure status changes
	t2 := builder.AppendOn(t1, 1)
	require.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t2.Key(), heightFromTip(t, t2)), false))
	s2 := syncer.Status()
	assert.Equal(t, false, s2.SyncingTrusted)

	assert.Equal(t, t2.Key(), s2.FetchingHead)
	assert.Equal(t, uint64(2), s2.FetchingHeight)

	assert.Equal(t, true, s2.SyncingFetchComplete)
	assert.Equal(t, true, s2.SyncingComplete)

	// do not advance the chain head, ensure valid head remains constant but fetching head changes
	t1 = builder.AppendOn(genesis, 1)
	require.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t1.Key(), heightFromTip(t, t1)), false))
	s2 = syncer.Status()
	assert.Equal(t, false, s2.SyncingTrusted)

	assert.Equal(t, t1.Key(), s2.FetchingHead)
	assert.Equal(t, uint64(1), s2.FetchingHeight)

	assert.Equal(t, true, s2.SyncingFetchComplete)
	assert.Equal(t, true, s2.SyncingComplete)
}

func TestStoresMessageReceipts(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, store, syncer := setup(ctx, t)
	genesis := builder.RequireTipSet(store.GetHead())

	keys := types.MustGenerateKeyInfo(1, 42)
	mm := types.NewMessageMaker(t, keys)
	alice := mm.Addresses()[0]
	t1 := builder.Build(genesis, 4, func(b *chain.BlockBuilder, i int) {
		b.AddMessages([]*types.SignedMessage{}, []*types.UnsignedMessage{mm.NewUnsignedMessage(alice, uint64(i))})
	})
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo(peer.ID(""), "", t1.Key(), heightFromTip(t, t1)), false))

	receiptsCid, err := store.GetTipSetReceiptsRoot(t1.Key())
	require.NoError(t, err)

	receipts, err := builder.LoadReceipts(ctx, receiptsCid)
	require.NoError(t, err)

	assert.Len(t, receipts, 4)
}

///// Set-up /////

// Initializes a chain builder, store and syncer.
// The chain builder has a single genesis block, which is set as the head of the store.
func setup(ctx context.Context, t *testing.T) (*chain.Builder, *chain.Store, *syncer.Syncer) {
	eval := &chain.FakeStateEvaluator{}
	return setupWithValidator(ctx, t, eval, eval)
}

func setupWithValidator(ctx context.Context, t *testing.T, fullVal syncer.FullBlockValidator, headerVal syncer.HeaderValidator) (*chain.Builder, *chain.Store, *syncer.Syncer) {
	builder := chain.NewBuilder(t, address.Undef)
	genesis := builder.NewGenesis()
	genStateRoot, err := builder.GetTipSetStateRoot(genesis.Key())
	require.NoError(t, err)

	store := chain.NewStore(repo.NewInMemoryRepo().ChainDatastore(), hamt.NewCborStore(), state.NewTreeLoader(), chain.NewStatusReporter(), genesis.At(0).Cid())
	// Initialize chainStore store genesis state and tipset as head.
	require.NoError(t, store.PutTipSetMetadata(ctx, &chain.TipSetMetadata{TipSetStateRoot: genStateRoot, TipSet: genesis, TipSetReceipts: types.EmptyReceiptsCID}))
	require.NoError(t, store.SetHead(ctx, genesis))

	// Note: the chain builder is passed as the fetcher, from which blocks may be requested, but
	// *not* as the store, to which the syncer must ensure to put blocks.
	sel := &chain.FakeChainSelector{}
	syncer, err := syncer.NewSyncer(fullVal, headerVal, sel, store, builder, builder, status.NewReporter(), th.NewFakeClock(time.Unix(1234567890, 0)), &noopFaultDetector{})
	require.NoError(t, err)
	require.NoError(t, syncer.InitStaged())

	return builder, store, syncer
}

///// Verification helpers /////

// Sub-interface of the store used for verification.
type syncStoreReader interface {
	GetHead() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
	GetTipSetStateRoot(tsKey block.TipSetKey) (cid.Cid, error)
	GetTipSetAndStatesByParentsAndHeight(block.TipSetKey, uint64) ([]*chain.TipSetMetadata, error)
}

// Verifies that a tipset and associated state root are stored in the chain store.
func verifyTip(t *testing.T, store syncStoreReader, tip block.TipSet, stateRoot cid.Cid) {
	foundTip, err := store.GetTipSet(tip.Key())
	require.NoError(t, err)
	assert.Equal(t, tip, foundTip)

	foundState, err := store.GetTipSetStateRoot(tip.Key())
	require.NoError(t, err)
	assert.Equal(t, stateRoot, foundState)

	parent, err := tip.Parents()
	assert.NoError(t, err)
	h, err := tip.Height()
	assert.NoError(t, err)
	childTsasSlice, err := store.GetTipSetAndStatesByParentsAndHeight(parent, h)
	assert.NoError(t, err)
	assert.True(t, containsTipSet(childTsasSlice, tip))
}

// Verifies that the store's head is as expected.
func verifyHead(t *testing.T, store syncStoreReader, head block.TipSet) {
	headTipSet, err := store.GetTipSet(store.GetHead())
	require.NoError(t, err)
	assert.Equal(t, head, headTipSet)
}

func containsTipSet(tsasSlice []*chain.TipSetMetadata, ts block.TipSet) bool {
	for _, tsas := range tsasSlice {
		if tsas.TipSet.String() == ts.String() { //bingo
			return true
		}
	}
	return false
}
