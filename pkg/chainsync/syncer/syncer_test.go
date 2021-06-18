package syncer_test

import (
	"context"
	"testing"
	"time"

	syncTypes "github.com/filecoin-project/venus/pkg/chainsync/types"
	emptycid "github.com/filecoin-project/venus/pkg/testhelpers/empty_cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/syncer"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/specactors/policy"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/test"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOneBlock(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, syncer := setup(ctx, t)
	t1 := builder.AppendOn(builder.Genesis(), 1)
	target := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", t1),
	}
	assert.NoError(t, syncer.HandleNewTipSet(ctx, target))

	verifyTip(t, builder.Store(), t1, t1.At(0).ParentStateRoot)
	verifyHead(t, builder.Store(), t1)
}

func TestMultiBlockTip(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, syncer := setup(ctx, t)
	genesis := builder.Store().GetHead()

	tip := builder.AppendOn(genesis, 2)
	target := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", tip),
	}
	assert.NoError(t, syncer.HandleNewTipSet(ctx, target))

	verifyTip(t, builder.Store(), tip, builder.StateForKey(tip.Key()))
	verifyHead(t, builder.Store(), tip)
}

func TestChainIncremental(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, syncer := setup(ctx, t)
	genesis := builder.Store().GetHead()

	t1 := builder.AppendOn(genesis, 2)

	t2 := builder.AppendOn(t1, 3)

	t3 := builder.AppendOn(t2, 1)

	t4 := builder.AppendOn(t3, 2)

	target1 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", t1),
	}

	target2 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", t2),
	}

	target3 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", t3),
	}
	target4 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", t4),
	}
	assert.NoError(t, syncer.HandleNewTipSet(ctx, target1))
	verifyTip(t, builder.Store(), t1, builder.StateForKey(t1.Key()))
	verifyHead(t, builder.Store(), t1)

	assert.NoError(t, syncer.HandleNewTipSet(ctx, target2))
	verifyTip(t, builder.Store(), t2, builder.StateForKey(t2.Key()))
	verifyHead(t, builder.Store(), t2)

	assert.NoError(t, syncer.HandleNewTipSet(ctx, target3))
	verifyTip(t, builder.Store(), t3, builder.StateForKey(t3.Key()))
	verifyHead(t, builder.Store(), t3)

	assert.NoError(t, syncer.HandleNewTipSet(ctx, target4))
	verifyTip(t, builder.Store(), t4, builder.StateForKey(t4.Key()))
	verifyHead(t, builder.Store(), t4)
}

func TestChainJump(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, syncer := setup(ctx, t)
	genesis := builder.Store().GetHead()

	t1 := builder.AppendOn(genesis, 2)
	t2 := builder.AppendOn(t1, 3)
	t3 := builder.AppendOn(t2, 1)
	t4 := builder.AppendOn(t3, 2)

	target1 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", t4),
	}
	assert.NoError(t, syncer.HandleNewTipSet(ctx, target1))
	verifyTip(t, builder.Store(), t1, builder.StateForKey(t1.Key()))
	verifyTip(t, builder.Store(), t2, builder.StateForKey(t2.Key()))
	verifyTip(t, builder.Store(), t3, builder.StateForKey(t3.Key()))
	verifyTip(t, builder.Store(), t4, builder.StateForKey(t4.Key()))
	verifyHead(t, builder.Store(), t4)
}

func TestIgnoreLightFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, syncer := setup(ctx, t)
	genesis := builder.Store().GetHead()

	forkbase := builder.AppendOn(genesis, 1)
	forkHead := builder.AppendOn(forkbase, 1)

	t1 := builder.AppendOn(forkbase, 1)
	t2 := builder.AppendOn(t1, 1)
	t3 := builder.AppendOn(t2, 1)
	t4 := builder.AppendOn(t3, 1)

	// Sync heaviest branch first.
	target4 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", t4),
	}
	assert.NoError(t, syncer.HandleNewTipSet(ctx, target4))
	verifyTip(t, builder.Store(), t4, builder.StateForKey(t4.Key()))
	verifyHead(t, builder.Store(), t4)

	// Lighter fork is processed but not change head.

	forkHeadTarget := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", forkHead),
	}
	assert.Error(t, syncer.HandleNewTipSet(ctx, forkHeadTarget))
	verifyHead(t, builder.Store(), t4)
}

func TestAcceptHeavierFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, syncer := setup(ctx, t)
	genesis := builder.Store().GetHead()

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

	main4Target := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", main4),
	}
	assert.NoError(t, syncer.HandleNewTipSet(ctx, main4Target))
	verifyTip(t, builder.Store(), main4, builder.StateForKey(main4.Key()))
	verifyHead(t, builder.Store(), main4)

	// Heavier fork updates head3
	fork3Target := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", fork3),
	}
	assert.NoError(t, syncer.HandleNewTipSet(ctx, fork3Target))
	verifyTip(t, builder.Store(), fork1, builder.StateForKey(fork1.Key()))
	verifyTip(t, builder.Store(), fork2, builder.StateForKey(fork2.Key()))
	verifyTip(t, builder.Store(), fork3, builder.StateForKey(fork3.Key()))
	verifyHead(t, builder.Store(), fork3)
}

func TestRejectFinalityFork(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, s := setup(ctx, t)
	genesis := builder.Store().GetHead()

	head := builder.AppendManyOn(int(policy.ChainFinality+2), genesis)
	target := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", head),
	}
	assert.NoError(t, s.HandleNewTipSet(ctx, target))

	// Differentiate fork for a new chain.  Fork has FinalityEpochs + 1
	// blocks on top of genesis so forkFinalityBase is more than FinalityEpochs
	// behind head
	forkFinalityBase := builder.BuildOneOn(genesis, func(bb *chain.BlockBuilder) {
		bb.SetTicket([]byte{0xbe})
	})
	forkFinalityHead := builder.AppendManyOn(int(policy.ChainFinality), forkFinalityBase)
	forkHeadTarget := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", forkFinalityHead),
	}
	assert.Error(t, s.HandleNewTipSet(ctx, forkHeadTarget))
}

func TestNoUncessesaryFetch(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, s := setup(ctx, t)
	genesis := builder.Store().GetHead()

	head := builder.AppendManyOn(4, genesis)
	target := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", head),
	}
	assert.NoError(t, s.HandleNewTipSet(ctx, target))

	// A new syncer unable to fetch blocks from the network can handle a tipset that's already
	// in the bsstore and linked to genesis.
	eval := &chain.FakeStateEvaluator{
		MessageStore: builder.Mstore(),
	}
	newSyncer, err := syncer.NewSyncer(eval,
		eval,
		&chain.FakeChainSelector{},
		builder.Store(),
		builder.Mstore(),
		builder.BlockStore(),
		builder,
		clock.NewFake(time.Unix(1234567890, 0)),
		fork.NewMockFork())
	require.NoError(t, err)

	target2 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", head),
	}
	err = newSyncer.HandleNewTipSet(ctx, target2)
	assert.Contains(t, err.Error(), "do not sync to a target has synced before")
}

// Syncer must track state of subsets of parent tipsets tracked in the bsstore
// when they are the ancestor in a chain.  This is in order to maintain the
// invariant that the aggregate state of the  parents of the base of a collected chain
// is kept in the bsstore.  This invariant allows chains built on subsets of
// tracked tipsets to be handled correctly.
// This test tests that the syncer stores the state of such a base tipset of a collected chain,
// i.e. a subset of an existing tipset in the bsstore.
//
// Ex: {A1, A2} -> {B1, B2, B3} in bsstore to start
// {B1, B2} -> {C1, C2} chain 1 input to syncer
// C1 -> D1 chain 2 input to syncer
//
// The last operation will fail if the state of subset {B1, B2} is not
// kept in the bsstore because syncing C1 requires retrieving parent state.
func TestSubsetParent(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, s := setup(ctx, t)
	genesis := builder.Store().GetHead()

	// Set up chain with {A1, A2} -> {B1, B2, B3}
	tipA1A2 := builder.AppendOn(genesis, 2)
	tipB1B2B3 := builder.AppendOn(tipA1A2, 3)
	target1 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", tipB1B2B3),
	}
	require.NoError(t, s.HandleNewTipSet(ctx, target1))

	// Sync one tipset with a parent equal to a subset of an existing
	// tipset in the bsstore: {B1, B2} -> {C1, C2}
	tipB1B2 := types.RequireNewTipSet(t, tipB1B2B3.At(0), tipB1B2B3.At(1))
	tipC1C2 := builder.AppendOn(tipB1B2, 2)

	target2 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", tipC1C2),
	}
	assert.NoError(t, s.HandleNewTipSet(ctx, target2))

	// Sync another tipset with a parent equal to a subset of the tipset
	// just synced: C1 -> D1
	tipC1 := types.RequireNewTipSet(t, tipC1C2.At(0))
	tipD1OnC1 := builder.AppendOn(tipC1, 1)

	target3 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", tipD1OnC1),
	}
	assert.NoError(t, s.HandleNewTipSet(ctx, target3))

	// A full parent also works fine: {C1, C2} -> D1
	tipD1OnC1C2 := builder.AppendOn(tipC1C2, 1)
	target4 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", tipD1OnC1C2),
	}
	assert.NoError(t, s.HandleNewTipSet(ctx, target4))
}

func TestBlockNotLinkedRejected(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	builder, syncer := setup(ctx, t)
	genesis := builder.Store().GetHead()

	// Set up a parallel builder from which the syncer cannot fetch.
	// The two builders are expected to produce exactly the same blocks from the same sequence
	// of calls.
	shadowBuilder := chain.NewBuilder(t, address.Undef)
	gen2 := shadowBuilder.Genesis()
	require.True(t, genesis.Equals(gen2))

	// The syncer fails to fetch this block so cannot sync it.
	b1 := shadowBuilder.AppendOn(genesis, 1)
	b2 := shadowBuilder.AppendOn(b1, 1)
	target1 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", b2),
	}
	assert.Error(t, syncer.HandleNewTipSet(ctx, target1))

	// Make the same block available from the syncer's builder
	builder.AppendBlockOn(genesis)
	target2 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", b1),
	}
	assert.NoError(t, syncer.HandleNewTipSet(ctx, target2))
}

type poisonValidator struct {
	headerFailureTS uint64
	fullFailureTS   uint64
}

func (pv *poisonValidator) RunStateTransition(ctx context.Context, ts *types.TipSet, parentStateRoot cid.Cid) (cid.Cid, cid.Cid, error) {
	stamp := ts.At(0).Timestamp
	if pv.fullFailureTS == stamp {
		return emptycid.EmptyTxMetaCID, emptycid.EmptyTxMetaCID, errors.New("run state transition fails on poison timestamp")
	}
	return emptycid.EmptyTxMetaCID, emptycid.EmptyTxMetaCID, nil
}

func (pv *poisonValidator) ValidateFullBlock(ctx context.Context, blk *types.BlockHeader) error {
	if pv.headerFailureTS == blk.Timestamp {
		return errors.New("val semantic fails on poison timestamp")
	}
	return nil
}

func newPoisonValidator(t *testing.T, headerFailure, fullFailure uint64) *poisonValidator {
	return &poisonValidator{headerFailureTS: headerFailure, fullFailureTS: fullFailure}
}

func (pv *poisonValidator) ValidateHeaderSemantic(_ context.Context, header *types.BlockHeader, _ *types.TipSet) error {
	if pv.headerFailureTS == header.Timestamp {
		return errors.New("val semantic fails on poison timestamp")
	}
	return nil
}

// ValidateHeaderSemantic is a stub that always returns no error
func (pv *poisonValidator) ValidateMessagesSemantic(_ context.Context, _ *types.BlockHeader, _ *types.TipSet) error {
	return nil
}

func TestSemanticallyBadTipSetFails(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	eval := newPoisonValidator(t, 98, 99)
	builder := chain.NewBuilder(t, address.Undef)
	builder, syncer := setupWithValidator(ctx, t, builder, eval, eval)
	genesis := builder.Store().GetHead()

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
	target1 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", link1),
	}
	err := syncer.HandleNewTipSet(ctx, target1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "val semantic fails")
}

// TODO: fix test
func TestStoresMessageReceipts(t *testing.T) {
	t.SkipNow()
	tf.UnitTest(t)
	ctx := context.Background()
	builder, syncer := setup(ctx, t)
	genesis := builder.Store().GetHead()

	keys := types.MustGenerateKeyInfo(1, 42)
	mm := types.NewMessageMaker(t, keys)
	alice := mm.Addresses()[0]
	t1 := builder.Build(genesis, 4, func(b *chain.BlockBuilder, i int) {
		b.AddMessages([]*types.SignedMessage{}, []*types.UnsignedMessage{mm.NewUnsignedMessage(alice, uint64(i))})
	})

	target1 := &syncTypes.Target{
		Base:      nil,
		Current:   nil,
		Start:     time.Time{},
		End:       time.Time{},
		Err:       nil,
		ChainInfo: *types.NewChainInfo("", "", t1),
	}
	assert.NoError(t, syncer.HandleNewTipSet(ctx, target1))

	receiptsCid, err := builder.Store().GetTipSetReceiptsRoot(t1)

	require.NoError(t, err)

	receipts, err := builder.LoadReceipts(ctx, receiptsCid)
	require.NoError(t, err)

	//filter same nonce
	assert.Len(t, receipts, 2)
}

///// Set-up /////

// Initializes a chain builder, bsstore and syncer.
// The chain builder has a single genesis block, which is set as the head of the bsstore.
func setup(ctx context.Context, t *testing.T) (*chain.Builder, *syncer.Syncer) {
	builder := chain.NewBuilder(t, address.Undef)
	eval := &chain.FakeStateEvaluator{
		MessageStore: builder.Mstore(),
	}
	return setupWithValidator(ctx, t, builder, eval, eval)
}

func setupWithValidator(ctx context.Context, t *testing.T, builder *chain.Builder, fullVal syncer.StateProcessor, headerVal syncer.BlockValidator) (*chain.Builder, *syncer.Syncer) {
	// Note: the chain builder is passed as the fetcher, from which blocks may be requested, but
	// *not* as the bsstore, to which the syncer must ensure to put blocks.
	sel := &chain.FakeChainSelector{}
	syncer, err := syncer.NewSyncer(fullVal,
		headerVal,
		sel,
		builder.Store(),
		builder.Mstore(),
		builder.BlockStore(),
		builder,
		clock.NewFake(time.Unix(1234567890, 0)),
		fork.NewMockFork())
	require.NoError(t, err)

	return builder, syncer
}

///// Verification helpers /////

// Sub-interface of the bsstore used for verification.
type syncStoreReader interface {
	GetHead() *types.TipSet
	GetTipSet(types.TipSetKey) (*types.TipSet, error)
	GetTipSetStateRoot(*types.TipSet) (cid.Cid, error)
	GetSiblingState(*types.TipSet) ([]*chain.TipSetMetadata, error)
}

// Verifies that a tipset and associated state root are stored in the chain bsstore.
func verifyTip(t *testing.T, store syncStoreReader, tip *types.TipSet, stateRoot cid.Cid) {
	foundTip, err := store.GetTipSet(tip.Key())
	require.NoError(t, err)
	test.Equal(t, tip, foundTip)

	foundState, err := store.GetTipSetStateRoot(tip)
	require.NoError(t, err)
	test.Equal(t, stateRoot, foundState)

	childTsasSlice, err := store.GetSiblingState(tip)
	assert.NoError(t, err)
	assert.True(t, containsTipSet(childTsasSlice, tip))
}

// Verifies that the bsstore's head is as expected.
func verifyHead(t *testing.T, store syncStoreReader, head *types.TipSet) {
	headTipSet := store.GetHead()
	test.Equal(t, head, headTipSet)
}

func containsTipSet(tsasSlice []*chain.TipSetMetadata, ts *types.TipSet) bool {
	for _, tsas := range tsasSlice {
		if tsas.TipSet.String() == ts.String() { //bingo
			return true
		}
	}
	return false
}
