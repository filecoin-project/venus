package syncer_test

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/internal/syncer"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/status"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Syncer is capable of recovering from a fork reorg after the store is loaded.
// This is a regression test to guard against the syncer assuming that the store having all
// blocks from a tipset means the syncer has computed its state.
// Such a case happens when the store has just loaded, but this tipset is not on its heaviest chain).
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
	cborStore := hamt.CSTFromBstore(bs)
	store := chain.NewStore(repo.ChainDatastore(), cborStore, state.NewTreeLoader(), chain.NewStatusReporter(), genesis.At(0).Cid())
	require.NoError(t, store.PutTipSetMetadata(ctx, &chain.TipSetMetadata{TipSetStateRoot: genStateRoot, TipSet: genesis, TipSetReceipts: types.EmptyReceiptsCID}))
	require.NoError(t, store.SetHead(ctx, genesis))

	// Note: the chain builder is passed as the fetcher, from which blocks may be requested, but
	// *not* as the store, to which the syncer must ensure to put blocks.
	eval := &chain.FakeStateEvaluator{}
	sel := &chain.FakeChainSelector{}
	s, err := syncer.NewSyncer(eval, eval, sel, store, builder, builder, status.NewReporter(), th.NewFakeClock(time.Unix(1234567890, 0)))
	require.NoError(t, err)
	require.NoError(t, s.StageHead())

	base := builder.AppendManyOn(3, genesis)
	left := builder.AppendManyOn(4, base)
	right := builder.AppendManyOn(3, base)

	// Sync the two branches, which stores all blocks in the underlying stores.
	assert.NoError(t, s.HandleNewTipSet(ctx, block.NewChainInfo("", "", left.Key(), heightFromTip(t, left))))
	assert.NoError(t, s.HandleNewTipSet(ctx, block.NewChainInfo("", "", right.Key(), heightFromTip(t, right))))
	verifyHead(t, store, left)

	// The syncer/store assume that the fetcher populates the underlying block store such that
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

	// Load a new chain store on the underlying data. It will only compute state for the
	// left (heavy) branch. It has a fetcher that can't provide blocks.
	newStore := chain.NewStore(repo.ChainDatastore(), cborStore, state.NewTreeLoader(), chain.NewStatusReporter(), genesis.At(0).Cid())
	require.NoError(t, newStore.Load(ctx))
	fakeFetcher := th.NewTestFetcher()
	offlineSyncer, err := syncer.NewSyncer(eval, eval, sel, newStore, builder, fakeFetcher, status.NewReporter(), th.NewFakeClock(time.Unix(1234567890, 0)))
	require.NoError(t, err)
	require.NoError(t, offlineSyncer.StageHead())

	assert.True(t, newStore.HasTipSetAndState(ctx, left.Key()))
	assert.False(t, newStore.HasTipSetAndState(ctx, right.Key()))

	// The newRight head extends right. The store already has the individual blocks up to the point
	// `right`, but has not computed their state (because it's not the heavy branch).
	// Obtuse code organisation means that the syncer will
	// attempt to fetch `newRight` *and `right`* blocks from the network in the process of computing
	// the state sequence for them all. Yes, this is a bit silly - the `right` blocks are already local.
	// The test is guarding against a prior incorrect behaviour where the syncer would not attempt to
	// fetch the `right` blocks (because it already has them) but *also* would not compute their state.
	// We detect this by making the final `newRight` blocks fetchable, but not the `right` blocks, and
	// expect the syncer to fail due to that failed fetch.
	// This test would fail to work if the syncer could inspect the store directly to avoid requesting
	// blocks already local, but also correctly recomputed the state.

	// Note that since the blocks are in the store, and a real fetcher will consult the store before
	// trying the network, this won't actually cause a network request. But it's really hard to follow.
	newRight := builder.AppendManyOn(1, right)
	fakeFetcher.AddSourceBlocks(newRight.ToSlice()...)

	// Test that the syncer can't sync a block chained from on the right (originally shorter) chain
	// without getting old blocks from network. i.e. the store index has been trimmed
	// of non-heaviest chain blocks.

	err = offlineSyncer.HandleNewTipSet(ctx, block.NewChainInfo("", "", newRight.Key(), heightFromTip(t, newRight)))
	assert.Error(t, err)

	// The left chain is ok without any fetching though.
	assert.NoError(t, offlineSyncer.HandleNewTipSet(ctx, block.NewChainInfo("", "", left.Key(), heightFromTip(t, left))))
}

// Power table weight comparisons impact syncer's selection.
// One fork has more blocks but less total power.
// Verify that the heavier fork is the one with more power.
// All blocks in this test follow protocol version 1 upgrade weighting rules.
func TestSyncerWeighsPower(t *testing.T) {
	cst := hamt.NewCborStore()
	ctx := context.Background()
	isb := newIntegrationStateBuilder(t, cst)
	builder := chain.NewBuilderWithState(t, address.Undef, isb)

	// Construct genesis with readable state tree root
	gen := builder.BuildOneOn(block.UndefTipSet, func(bb *chain.BlockBuilder) {})

	// Builder constructs two different blocks with different state trees
	// for building two forks.
	split := builder.BuildOn(gen, 2, func(bb *chain.BlockBuilder, i int) {
		if i == 1 {
			keys := types.MustGenerateKeyInfo(1, 42)
			mm := types.NewMessageMaker(t, keys)
			addr := mm.Addresses()[0]
			bb.AddMessages(
				[]*types.SignedMessage{mm.NewSignedMessage(addr, 1)},
				[]*types.UnsignedMessage{},
			)
		}
	})
	fork1 := th.RequireNewTipSet(t, split.At(0))
	fork2 := th.RequireNewTipSet(t, split.At(1))

	// Builder adds 3 blocks to fork 1 and total storage power 2^0
	// 3 + 3*delta = 3 + 3[V*1 + bits(2^0)] = 3 + 3[2 + 1] = 3 + 9 = 12
	head1 := builder.AppendManyOn(3, fork1)

	// Builder adds 1 block to fork 2 and total storage power 2^9
	// 3 + 1*delta = 3 + 1[V*1 + bits(2^9)] = 3 + 2 + 10 = 15
	head2 := builder.AppendOn(fork2, 1)

	// Verify that the syncer selects fork 2 (15 > 12)
	as := newForkSnapshotGen(t, types.NewBytesAmount(1), types.NewBytesAmount(512), isb.c512)
	dumpBlocksToCborStore(t, builder, cst, head1, head2)
	store := chain.NewStore(repo.NewInMemoryRepo().ChainDatastore(), cst, state.NewTreeLoader(), chain.NewStatusReporter(), gen.At(0).Cid())
	require.NoError(t, store.PutTipSetMetadata(ctx, &chain.TipSetMetadata{TipSetStateRoot: gen.At(0).StateRoot, TipSet: gen, TipSetReceipts: gen.At(0).MessageReceipts}))
	require.NoError(t, store.SetHead(ctx, gen))
	eval := &integrationStateEvaluator{c512: isb.c512}
	syncer, err := syncer.NewSyncer(eval, eval, consensus.NewChainSelector(cst, as, gen.At(0).Cid()), store, builder, builder, status.NewReporter(), th.NewFakeClock(time.Unix(1234567890, 0)))
	require.NoError(t, err)
	require.NoError(t, syncer.StageHead())

	// sync fork 1
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo("", "", head1.Key(), heightFromTip(t, head1))))
	assert.Equal(t, head1.Key(), store.GetHead())
	// sync fork 2
	assert.NoError(t, syncer.HandleNewTipSet(ctx, block.NewChainInfo("", "", head2.Key(), heightFromTip(t, head1))))
	assert.Equal(t, head2.Key(), store.GetHead())
}

// integrationStateBuilder is a chain/testing.go `StateBuilder` used for
// construction of a chain where the state root cids signify the total power
// in the power table without actually needing to construct a valid state
// state machine tree.
//
// All blocks with at least one message are assigned a special cid: c512.
// In TestSyncerWeighsPower this state root is interpreted as having
// 512 bytes of power.
//
// integrationStateBuilder also weighs the chain according to the protocol
// version 1 upgrade.
type integrationStateBuilder struct {
	t    *testing.T
	c512 cid.Cid
	cGen cid.Cid
	cst  *hamt.CborIpldStore
}

func newIntegrationStateBuilder(t *testing.T, cst *hamt.CborIpldStore) *integrationStateBuilder {
	return &integrationStateBuilder{
		t:    t,
		c512: cid.Undef,
		cst:  cst,
		cGen: cid.Undef,
	}
}

func (isb *integrationStateBuilder) ComputeState(prev cid.Cid, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage) (cid.Cid, []*types.MessageReceipt, error) {
	// setup genesis with a state we can fetch from cborstor
	if prev.Equals(types.CidFromString(isb.t, "null")) {
		treeGen := state.TreeFromString(isb.t, "1Power", isb.cst)
		genRoot, err := treeGen.Flush(context.Background())
		require.NoError(isb.t, err)
		return genRoot, []*types.MessageReceipt{}, nil
	}
	// Setup fork with state we associate with more power.
	// This fork is distiguished by a block with a single secp message.
	if len(secpMessages[0]) > 0 {
		treeFork := state.TreeFromString(isb.t, "512Power", isb.cst)
		forkRoot, err := treeFork.Flush(context.Background())
		require.NoError(isb.t, err)
		isb.c512 = forkRoot
		return forkRoot, []*types.MessageReceipt{}, nil
	}
	return prev, []*types.MessageReceipt{}, nil
}

func (isb *integrationStateBuilder) Weigh(tip block.TipSet, pstate cid.Cid) (uint64, error) {
	if tip.Equals(block.UndefTipSet) {
		return uint64(0), nil
	}
	if isb.cGen.Equals(cid.Undef) && tip.Len() == 1 {
		isb.cGen = tip.At(0).Cid()
	}

	if tip.At(0).Cid().Equals(isb.cGen) {
		return uint64(0), nil
	}
	as := newForkSnapshotGen(isb.t, types.NewBytesAmount(1), types.NewBytesAmount(512), isb.c512)
	sel := consensus.NewChainSelector(isb.cst, as, isb.cGen)
	return sel.Weight(context.Background(), tip, pstate)
}

// integrationStateEvaluator returns the parent state root.  If there are multiple
// parent blocks and any contain state root c512 then it will return c512.
type integrationStateEvaluator struct {
	c512 cid.Cid
}

func (n *integrationStateEvaluator) RunStateTransition(_ context.Context, ts block.TipSet, _ [][]*types.UnsignedMessage, _ [][]*types.SignedMessage, _ []block.TipSet, _ uint64, stateID cid.Cid, rCid cid.Cid) (cid.Cid, []*types.MessageReceipt, error) {
	for i := 0; i < ts.Len(); i++ {
		if ts.At(i).StateRoot.Equals(n.c512) {
			return n.c512, []*types.MessageReceipt{}, nil
		}
	}
	return ts.At(0).StateRoot, []*types.MessageReceipt{}, nil
}

func (n *integrationStateEvaluator) ValidateSemantic(_ context.Context, _ *block.Block, _ block.TipSet) error {
	return nil
}

// forkSnapshotGen reads power from fake state tree root cids.  It reads
// power of `forkPower` from cid `forkRoot` and `defaultPower` from all others.
type forkSnapshotGen struct {
	forkPower    *types.BytesAmount
	defaultPower *types.BytesAmount
	forkRoot     cid.Cid
	t            *testing.T
}

func newForkSnapshotGen(t *testing.T, dp, fp *types.BytesAmount, root cid.Cid) *forkSnapshotGen {
	return &forkSnapshotGen{
		t:            t,
		defaultPower: dp,
		forkPower:    fp,
		forkRoot:     root,
	}
}

func (fs *forkSnapshotGen) StateTreeSnapshot(st state.Tree, bh *types.BlockHeight) consensus.ActorStateSnapshot {
	totalPower := fs.defaultPower

	root, err := st.Flush(context.Background())
	require.NoError(fs.t, err)
	if root.Equals(fs.forkRoot) {
		totalPower = fs.forkPower
	}

	return &consensus.FakePowerTableViewSnapshot{
		MinerPower:    types.NewBytesAmount(0),
		TotalPower:    totalPower,
		MinerToWorker: make(map[address.Address]address.Address),
	}
}

// dumpBlocksToCborStore is a helper method that
// TODO #3078 we can avoid this byte shuffling by creating a simple testing type
// that implements the needed interface and grabs blocks from the builder as
// needed.  Once #3078 is in place we will have the flexibility to use a
// testing type as the cbor store.
func dumpBlocksToCborStore(t *testing.T, builder *chain.Builder, cst *hamt.CborIpldStore, heads ...block.TipSet) {
	cids := make(map[cid.Cid]struct{})
	// traverse builder frontier adding cids to the map. Traverse
	// duplicates over doing anything clever.
	var err error
	for _, head := range heads {
		it := chain.IterAncestors(context.Background(), builder, head)
		for ; !it.Complete(); err = it.Next() {
			require.NoError(t, err)
			for i := 0; i < it.Value().Len(); i++ {
				blk := head.At(i)
				c := blk.Cid()
				cids[c] = struct{}{}
			}
		}
	}

	// get all blocks corresponding to the cids and put to the cst
	var searchKey []cid.Cid
	for c := range cids {
		searchKey = append(searchKey, c)
	}
	blocks, err := builder.GetBlocks(context.Background(), searchKey)
	require.NoError(t, err)
	for _, blk := range blocks {
		_, err = cst.Put(context.Background(), blk)
		require.NoError(t, err)
	}
}
