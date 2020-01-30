package chain_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ipfs/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// Default Chain diagram below.  Note that blocks in the same tipset are in parentheses.
//
// genesis -> (link1blk1, link1blk2) -> (link2blk1, link2blk2, link2blk3) -> link3blk1 -> (null block) -> (null block) -> (link4blk1, link4blk2)

// newChainStore creates a new chain store for tests.
func newChainStore(r repo.Repo, genCid cid.Cid) *chain.Store {
	return chain.NewStore(r.Datastore(), hamt.NewCborStore(), state.NewTreeLoader(), chain.NewStatusReporter(), genCid)
}

// requirePutTestChain puts the count tipsets preceding head in the source to
// the input chain store.
func requirePutTestChain(ctx context.Context, t *testing.T, chainStore *chain.Store, head block.TipSetKey, source *chain.Builder, count int) {
	tss := source.RequireTipSets(head, count)
	for _, ts := range tss {
		tsas := &chain.TipSetMetadata{
			TipSet:          ts,
			TipSetStateRoot: ts.At(0).StateRoot,
			TipSetReceipts:  types.EmptyReceiptsCID,
		}
		require.NoError(t, chainStore.PutTipSetMetadata(ctx, tsas))
	}
}

func requireGetTsasByParentAndHeight(t *testing.T, chain *chain.Store, pKey block.TipSetKey, h uint64) []*chain.TipSetMetadata {
	tsasSlice, err := chain.GetTipSetAndStatesByParentsAndHeight(pKey, h)
	require.NoError(t, err)
	return tsasSlice
}

type HeadAndTipsetGetter interface {
	GetHead() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
}

func requirePutBlocksToCborStore(t *testing.T, cst hamt.CborIpldStore, blocks ...*block.Block) {
	for _, block := range blocks {
		_, err := cst.Put(context.Background(), block)
		require.NoError(t, err)
	}
}

/* Putting and getting tipsets and states. */

// Adding tipsets to the store doesn't error.
func TestPutTipSet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS.At(0).Cid())

	genTsas := &chain.TipSetMetadata{
		TipSet:          genTS,
		TipSetStateRoot: genTS.At(0).StateRoot,
		TipSetReceipts:  types.EmptyReceiptsCID,
	}
	err := cs.PutTipSetMetadata(ctx, genTsas)
	assert.NoError(t, err)
}

// Tipsets can be retrieved by key (all block cids).
func TestGetByKey(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS.At(0).Cid())

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Put the test chain to the store
	requirePutTestChain(ctx, t, cs, link4.Key(), builder, 5)

	// Check that we can get all tipsets by key
	gotGTS := requireGetTipSet(ctx, t, cs, genTS.Key())
	gotGTSSR := requireGetTipSetStateRoot(ctx, t, cs, genTS.Key())

	got1TS := requireGetTipSet(ctx, t, cs, link1.Key())
	got1TSSR := requireGetTipSetStateRoot(ctx, t, cs, link1.Key())

	got2TS := requireGetTipSet(ctx, t, cs, link2.Key())
	got2TSSR := requireGetTipSetStateRoot(ctx, t, cs, link2.Key())

	got3TS := requireGetTipSet(ctx, t, cs, link3.Key())
	got3TSSR := requireGetTipSetStateRoot(ctx, t, cs, link3.Key())

	got4TS := requireGetTipSet(ctx, t, cs, link4.Key())
	got4TSSR := requireGetTipSetStateRoot(ctx, t, cs, link4.Key())
	assert.Equal(t, genTS, gotGTS)
	assert.Equal(t, link1, got1TS)
	assert.Equal(t, link2, got2TS)
	assert.Equal(t, link3, got3TS)
	assert.Equal(t, link4, got4TS)

	assert.Equal(t, genTS.At(0).StateRoot, gotGTSSR)
	assert.Equal(t, link1.At(0).StateRoot, got1TSSR)
	assert.Equal(t, link2.At(0).StateRoot, got2TSSR)
	assert.Equal(t, link3.At(0).StateRoot, got3TSSR)
	assert.Equal(t, link4.At(0).StateRoot, got4TSSR)
}

// Tipset state is loaded correctly
func TestGetTipSetState(t *testing.T) {
	ctx := context.Background()
	cst := hamt.NewCborStore()

	// setup testing state
	fakeCode := types.CidFromString(t, "somecid")
	balance := types.NewAttoFILFromFIL(1000000)
	testActor := actor.NewActor(fakeCode, balance)
	addr := address.NewForTestGetter()()
	st1 := state.NewTree(cst)
	require.NoError(t, st1.SetActor(ctx, addr, testActor))
	root, err := st1.Flush(ctx)
	require.NoError(t, err)

	// link testing state to test block
	builder := chain.NewBuilder(t, address.Undef)
	gen := builder.NewGenesis()
	testTs := builder.BuildOneOn(gen, func(b *chain.BlockBuilder) {
		b.SetStateRoot(root)
	})

	// setup chain store
	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	store := chain.NewStore(ds, cst, state.NewTreeLoader(), chain.NewStatusReporter(), gen.At(0).Cid())

	// add tipset and state to chain store
	require.NoError(t, store.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
		TipSet:          testTs,
		TipSetStateRoot: root,
		TipSetReceipts:  types.EmptyReceiptsCID,
	}))

	// verify output of GetTipSetState
	st2, err := store.GetTipSetState(ctx, testTs.Key())
	assert.NoError(t, err)
	for actRes := range st2.GetAllActors(ctx) {
		assert.NoError(t, actRes.Error)
		assert.Equal(t, addr.String(), actRes.Address)
		assert.Equal(t, fakeCode, actRes.Actor.Code)
		assert.Equal(t, testActor.Head, actRes.Actor.Head)
		assert.Equal(t, types.Uint64(0), actRes.Actor.CallSeqNum)
		assert.Equal(t, balance, actRes.Actor.Balance)
	}
}

// Tipsets can be retrieved by parent key (all block cids of parents).
func TestGetByParent(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS.At(0).Cid())

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Put the test chain to the store
	requirePutTestChain(ctx, t, cs, link4.Key(), builder, 5)

	gotG := requireGetTsasByParentAndHeight(t, cs, block.TipSetKey{}, uint64(0))
	got1 := requireGetTsasByParentAndHeight(t, cs, genTS.Key(), uint64(1))
	got2 := requireGetTsasByParentAndHeight(t, cs, link1.Key(), uint64(2))
	got3 := requireGetTsasByParentAndHeight(t, cs, link2.Key(), uint64(3))
	got4 := requireGetTsasByParentAndHeight(t, cs, link3.Key(), uint64(6)) // two null blocks in between 3 and 4!

	assert.Equal(t, genTS, gotG[0].TipSet)
	assert.Equal(t, link1, got1[0].TipSet)
	assert.Equal(t, link2, got2[0].TipSet)
	assert.Equal(t, link3, got3[0].TipSet)
	assert.Equal(t, link4, got4[0].TipSet)

	assert.Equal(t, genTS.At(0).StateRoot, gotG[0].TipSetStateRoot)
	assert.Equal(t, link1.At(0).StateRoot, got1[0].TipSetStateRoot)
	assert.Equal(t, link2.At(0).StateRoot, got2[0].TipSetStateRoot)
	assert.Equal(t, link3.At(0).StateRoot, got3[0].TipSetStateRoot)
	assert.Equal(t, link4.At(0).StateRoot, got4[0].TipSetStateRoot)
}

func TestGetMultipleByParent(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS.At(0).Cid())

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Put the test chain to the store
	requirePutTestChain(ctx, t, cs, link4.Key(), builder, 5)

	// Add extra children to the genesis tipset
	otherLink1 := builder.AppendOn(genTS, 1)
	otherRoot1 := types.CidFromString(t, "otherState")
	newChildTsas := &chain.TipSetMetadata{
		TipSet:          otherLink1,
		TipSetStateRoot: otherRoot1,
		TipSetReceipts:  types.EmptyReceiptsCID,
	}
	require.NoError(t, cs.PutTipSetMetadata(ctx, newChildTsas))
	gotNew1 := requireGetTsasByParentAndHeight(t, cs, genTS.Key(), uint64(1))
	require.Equal(t, 2, len(gotNew1))
	for _, tsas := range gotNew1 {
		if tsas.TipSet.Len() == 1 {
			assert.Equal(t, otherRoot1, tsas.TipSetStateRoot)
		} else {
			assert.Equal(t, link1.At(0).StateRoot, tsas.TipSetStateRoot)
		}
	}
}

/* Head and its State is set and notified properly. */

// The constructor call sets the genesis cid for the chain store.
func TestSetGenesis(t *testing.T) {
	tf.UnitTest(t)

	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS.At(0).Cid())

	require.Equal(t, genTS.At(0).Cid(), cs.GenesisCid())
}

func assertSetHead(t *testing.T, chainStore *chain.Store, ts block.TipSet) {
	ctx := context.Background()
	err := chainStore.SetHead(ctx, ts)
	assert.NoError(t, err)
}

// Set and Get Head.
func TestHead(t *testing.T) {
	tf.UnitTest(t)

	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()
	sr := chain.NewStatusReporter()
	cs := chain.NewStore(r.Datastore(), hamt.NewCborStore(), state.NewTreeLoader(), sr, genTS.At(0).Cid())

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Head starts as an empty cid set
	assert.Equal(t, block.TipSetKey{}, cs.GetHead())

	// Set Head
	assertSetHead(t, cs, genTS)
	assert.Equal(t, genTS.Key(), cs.GetHead())
	assert.Equal(t, genTS.Key(), sr.Status().ValidatedHead)

	// Move head forward
	assertSetHead(t, cs, link4)
	assert.Equal(t, link4.Key(), cs.GetHead())
	assert.Equal(t, link4.Key(), sr.Status().ValidatedHead)

	// Move head back
	assertSetHead(t, cs, link1)
	assert.Equal(t, link1.Key(), cs.GetHead())
	assert.Equal(t, link1.Key(), sr.Status().ValidatedHead)
}

func assertEmptyCh(t *testing.T, ch <-chan interface{}) {
	select {
	case <-ch:
		assert.True(t, false)
	default:
	}
}

// Head events are propagated on HeadEvents.
func TestHeadEvents(t *testing.T) {
	tf.UnitTest(t)

	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()
	chainStore := newChainStore(r, genTS.At(0).Cid())

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })
	ps := chainStore.HeadEvents()
	chA := ps.Sub(chain.NewHeadTopic)
	chB := ps.Sub(chain.NewHeadTopic)

	assertSetHead(t, chainStore, genTS)
	assertSetHead(t, chainStore, link1)
	assertSetHead(t, chainStore, link2)
	assertSetHead(t, chainStore, link3)
	assertSetHead(t, chainStore, link4)
	assertSetHead(t, chainStore, link3)
	assertSetHead(t, chainStore, link2)
	assertSetHead(t, chainStore, link1)
	assertSetHead(t, chainStore, genTS)
	heads := []block.TipSet{genTS, link1, link2, link3, link4, link3, link2, link1, genTS}

	// Heads arrive in the expected order
	for i := 0; i < 9; i++ {
		headA := <-chA
		headB := <-chB
		assert.Equal(t, headA, headB)
		assert.Equal(t, headA, heads[i])
	}

	// No extra notifications
	assertEmptyCh(t, chA)
	assertEmptyCh(t, chB)
}

/* Loading  */
// Load does not error and gives the chain store access to all blocks and
// tipset indexes along the heaviest chain.
func TestLoadAndReboot(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.NewGenesis()
	rPriv := repo.NewInMemoryRepo()
	ds := rPriv.Datastore()
	cst := hamt.NewCborStore()

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Add blocks to blockstore
	requirePutBlocksToCborStore(t, cst, genTS.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link1.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link2.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link3.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link4.ToSlice()...)

	chainStore := chain.NewStore(ds, cst, state.NewTreeLoader(), chain.NewStatusReporter(), genTS.At(0).Cid())
	requirePutTestChain(ctx, t, chainStore, link4.Key(), builder, 5)
	assertSetHead(t, chainStore, genTS) // set the genesis block

	assertSetHead(t, chainStore, link4)
	chainStore.Stop()

	// rebuild chain with same datastore and cborstore
	sr := chain.NewStatusReporter()
	rebootChain := chain.NewStore(ds, cst, state.NewTreeLoader(), sr, genTS.At(0).Cid())
	err := rebootChain.Load(ctx)
	assert.NoError(t, err)
	assert.Equal(t, link4.Key(), sr.Status().ValidatedHead)

	// Check that chain store has index
	// Get a tipset and state by key
	got2 := requireGetTipSet(ctx, t, rebootChain, link2.Key())
	assert.Equal(t, link2, got2)

	// Get another by parent key
	got4 := requireGetTsasByParentAndHeight(t, rebootChain, link3.Key(), uint64(6))
	assert.Equal(t, 1, len(got4))
	assert.Equal(t, link4, got4[0].TipSet)

	// Check the head
	assert.Equal(t, link4.Key(), rebootChain.GetHead())
}

type tipSetGetter interface {
	GetTipSet(block.TipSetKey) (block.TipSet, error)
}

func requireGetTipSet(ctx context.Context, t *testing.T, chainStore tipSetGetter, key block.TipSetKey) block.TipSet {
	ts, err := chainStore.GetTipSet(key)
	require.NoError(t, err)
	return ts
}

type tipSetStateRootGetter interface {
	GetTipSetStateRoot(tsKey block.TipSetKey) (cid.Cid, error)
}

func requireGetTipSetStateRoot(ctx context.Context, t *testing.T, chainStore tipSetStateRootGetter, key block.TipSetKey) cid.Cid {
	stateCid, err := chainStore.GetTipSetStateRoot(key)
	require.NoError(t, err)
	return stateCid
}
