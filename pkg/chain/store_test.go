package chain_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/config"
	emptycid "github.com/filecoin-project/venus/pkg/testhelpers/empty_cid"
	"github.com/filecoin-project/venus/pkg/util/test"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/repo"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/types"
)

type CborBlockStore struct {
	*chain.Store
	cborStore cbor.IpldStore
}

func (cbor *CborBlockStore) PutBlocks(ctx context.Context, blocks []*types.BlockHeader) {
	for _, blk := range blocks {
		_, _ = cbor.cborStore.Put(ctx, blk)
	}

}

// Default Chain diagram below.  Note that blocks in the same tipset are in parentheses.
//
// genesis -> (link1blk1, link1blk2) -> (link2blk1, link2blk2, link2blk3) -> link3blk1 -> (null block) -> (null block) -> (link4blk1, link4blk2)

// newChainStore creates a new chain store for tests.
func newChainStore(r repo.Repo, genTS *types.TipSet) *CborBlockStore {
	tempBlock := r.Datastore()
	cborStore := cbor.NewCborStore(tempBlock)
	return &CborBlockStore{
		Store:     chain.NewStore(r.ChainDatastore(), cborStore, tempBlock, chain.NewStatusReporter(), config.DefaultForkUpgradeParam, genTS.At(0).Cid()),
		cborStore: cborStore,
	}
}

// requirePutTestChain puts the count tipsets preceding head in the source to
// the input chain store.
func requirePutTestChain(ctx context.Context, t *testing.T, cborStore *CborBlockStore, head types.TipSetKey, source *chain.Builder, count int) {
	tss := source.RequireTipSets(head, count)
	for _, ts := range tss {
		tsas := &chain.TipSetMetadata{
			TipSet:          ts,
			TipSetStateRoot: ts.At(0).ParentStateRoot,
			TipSetReceipts:  emptycid.EmptyReceiptsCID,
		}
		requirePutBlocksToCborStore(t, cborStore.cborStore, tsas.TipSet.Blocks()...)
		require.NoError(t, cborStore.PutTipSetMetadata(ctx, tsas))
	}
}

func requireSiblingState(t *testing.T, cborStore *CborBlockStore, ts *types.TipSet) []*chain.TipSetMetadata {
	if ts.Height() == 0 {
		tsasSlice, err := cborStore.LoadTipsetMetadata(ts)
		require.NoError(t, err)
		return []*chain.TipSetMetadata{tsasSlice}
	}
	tsasSlice, err := cborStore.GetSiblingState(ts)
	require.NoError(t, err)
	return tsasSlice
}

type HeadAndTipsetGetter interface {
	GetHead() types.TipSetKey
	GetTipSet(types.TipSetKey) (types.TipSet, error)
}

func requirePutBlocksToCborStore(t *testing.T, cst cbor.IpldStore, blocks ...*types.BlockHeader) {
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
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS)

	genTsas := &chain.TipSetMetadata{
		TipSet:          genTS,
		TipSetStateRoot: genTS.At(0).ParentStateRoot,
		TipSetReceipts:  emptycid.EmptyReceiptsCID,
	}
	err := cs.PutTipSetMetadata(ctx, genTsas)
	assert.NoError(t, err)
}

// Tipsets can be retrieved by key (all block cids).
func TestGetByKey(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS)

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Put the test chain to the store
	requirePutTestChain(ctx, t, cs, link4.Key(), builder, 5)

	// Check that we can get all tipsets by key
	gotGTS := requireGetTipSet(ctx, t, cs, genTS.Key())
	gotGTSSR := requireGetTipSetStateRoot(ctx, t, cs, genTS)

	got1TS := requireGetTipSet(ctx, t, cs, link1.Key())
	got1TSSR := requireGetTipSetStateRoot(ctx, t, cs, link1)

	got2TS := requireGetTipSet(ctx, t, cs, link2.Key())
	got2TSSR := requireGetTipSetStateRoot(ctx, t, cs, link2)

	got3TS := requireGetTipSet(ctx, t, cs, link3.Key())
	got3TSSR := requireGetTipSetStateRoot(ctx, t, cs, link3)

	got4TS := requireGetTipSet(ctx, t, cs, link4.Key())
	got4TSSR := requireGetTipSetStateRoot(ctx, t, cs, link4)
	assert.ObjectsAreEqualValues(genTS, gotGTS)
	assert.ObjectsAreEqualValues(link1, got1TS)
	assert.ObjectsAreEqualValues(link2, got2TS)
	assert.ObjectsAreEqualValues(link3, got3TS)
	assert.ObjectsAreEqualValues(link4, got4TS)

	assert.Equal(t, genTS.At(0).ParentStateRoot, gotGTSSR)
	assert.Equal(t, link1.At(0).ParentStateRoot, got1TSSR)
	assert.Equal(t, link2.At(0).ParentStateRoot, got2TSSR)
	assert.Equal(t, link3.At(0).ParentStateRoot, got3TSSR)
	assert.Equal(t, link4.At(0).ParentStateRoot, got4TSSR)
}

// Tipsets can be retrieved by key (all block cids).
func TestRevertChange(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.TODO()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	cs := newChainStore(builder.Repo(), genTS)
	genesis := builder.Genesis()

	link1 := builder.AppendOn(genesis, 1)
	link2 := builder.AppendOn(link1, 1)
	link3 := builder.AppendOn(link2, 1)

	err := cs.SetHead(ctx, link3)
	require.NoError(t, err)

	link4 := builder.AppendOn(genesis, 2)
	link5 := builder.AppendOn(link4, 2)
	link6 := builder.AppendOn(link5, 2)

	ch := cs.SubHeadChanges(ctx)
	currentA := <-ch
	test.Equal(t, currentA[0].Type, chain.HCCurrent)
	test.Equal(t, currentA[0].Val, link3)

	err = cs.SetHead(ctx, link6)
	require.NoError(t, err)
	headChanges := <-ch

	if len(headChanges) == 1 {
		//maybe link3, if link3 fetch next
		headChanges = <-ch
	}
	test.Equal(t, headChanges[0].Type, chain.HCRevert)
	test.Equal(t, headChanges[0].Val, link1)
	test.Equal(t, headChanges[1].Type, chain.HCRevert)
	test.Equal(t, headChanges[1].Val, link2)
	test.Equal(t, headChanges[2].Type, chain.HCRevert)
	test.Equal(t, headChanges[2].Val, link3)

	test.Equal(t, headChanges[3].Type, chain.HCApply)
	test.Equal(t, headChanges[3].Val, link4)
	test.Equal(t, headChanges[4].Type, chain.HCApply)
	test.Equal(t, headChanges[4].Val, link5)
	test.Equal(t, headChanges[5].Type, chain.HCApply)
	test.Equal(t, headChanges[5].Val, link6)
}

// Tipsets can be retrieved by parent key (all block cids of parents).
func TestGetByParent(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS)

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Put the test chain to the store
	requirePutTestChain(ctx, t, cs, link4.Key(), builder, 5)

	gotG := requireSiblingState(t, cs, genTS)
	got1 := requireSiblingState(t, cs, link1)
	got2 := requireSiblingState(t, cs, link2)
	got3 := requireSiblingState(t, cs, link3)
	got4 := requireSiblingState(t, cs, link4) // two null blocks in between 3 and 4!

	assert.ObjectsAreEqualValues(genTS, gotG[0].TipSet)
	assert.ObjectsAreEqualValues(link1, got1[0].TipSet)
	assert.ObjectsAreEqualValues(link2, got2[0].TipSet)
	assert.ObjectsAreEqualValues(link3, got3[0].TipSet)
	assert.ObjectsAreEqualValues(link4, got4[0].TipSet)

	assert.Equal(t, genTS.At(0).ParentStateRoot, gotG[0].TipSetStateRoot)
	assert.Equal(t, link1.At(0).ParentStateRoot, got1[0].TipSetStateRoot)
	assert.Equal(t, link2.At(0).ParentStateRoot, got2[0].TipSetStateRoot)
	assert.Equal(t, link3.At(0).ParentStateRoot, got3[0].TipSetStateRoot)
	assert.Equal(t, link4.At(0).ParentStateRoot, got4[0].TipSetStateRoot)
}

func TestGetMultipleByParent(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS)

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
		TipSetReceipts:  emptycid.EmptyReceiptsCID,
	}
	require.NoError(t, cs.PutTipSetMetadata(ctx, newChildTsas))
	gotNew1 := requireSiblingState(t, cs, otherLink1)
	require.Equal(t, 2, len(gotNew1))
	for _, tsas := range gotNew1 {
		if tsas.TipSet.Len() == 1 {
			assert.ObjectsAreEqualValues(otherRoot1, tsas.TipSetStateRoot)
		} else {
			assert.ObjectsAreEqualValues(link1.At(0).ParentStateRoot, tsas.TipSetStateRoot)
		}
	}
}

/* Head and its state is set and notified properly. */

// The constructor call sets the genesis cid for the chain store.
func TestSetGenesis(t *testing.T) {
	tf.UnitTest(t)

	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS)

	require.Equal(t, genTS.At(0).Cid(), cs.GenesisCid())
}

func assertSetHead(t *testing.T, cborStore *CborBlockStore, ts *types.TipSet) {
	ctx := context.Background()
	err := cborStore.SetHead(ctx, ts)
	assert.NoError(t, err)
}

// Set and Get Head.
func TestHead(t *testing.T) {
	tf.UnitTest(t)

	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := builder.Repo()
	sr := chain.NewStatusReporter()
	bs := builder.BlockStore()
	cborStore := builder.Cstore()
	cs := chain.NewStore(r.ChainDatastore(), cborStore, bs, sr, config.DefaultForkUpgradeParam, genTS.At(0).Cid())
	cboreStore := &CborBlockStore{
		Store: chain.NewStore(r.ChainDatastore(), cborStore, bs, sr, config.DefaultForkUpgradeParam, genTS.At(0).Cid()),
	}
	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Head starts as an empty cid set
	assert.Equal(t, types.UndefTipSet, cs.GetHead())

	// Set Head
	assertSetHead(t, cboreStore, genTS)
	assert.ObjectsAreEqualValues(genTS.Key(), cs.GetHead())
	assert.Equal(t, genTS.Key(), sr.Status().ValidatedHead)

	// Move head forward
	assertSetHead(t, cboreStore, link4)
	assert.ObjectsAreEqualValues(link4.Key(), cs.GetHead())
	assert.Equal(t, link4.Key(), sr.Status().ValidatedHead)

	// Move head back
	assertSetHead(t, cboreStore, link1)
	assert.ObjectsAreEqualValues(link1.Key(), cs.GetHead())
	assert.Equal(t, link1.Key(), sr.Status().ValidatedHead)
}

func assertEmptyCh(t *testing.T, ch <-chan []*chain.HeadChange) {
	select {
	case <-ch:
		assert.True(t, false)
	default:
	}
}

// Head events are propagated on HeadEvents.
func TestHeadEvents(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	chainStore := newChainStore(builder.Repo(), genTS)
	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	assertSetHead(t, chainStore, genTS)

	chA := chainStore.SubHeadChanges(ctx)
	chB := chainStore.SubHeadChanges(ctx)
	// HCurrent
	currentA := <-chA
	test.Equal(t, currentA[0].Type, chain.HCCurrent)
	test.Equal(t, currentA[0].Val, genTS)

	currentB := <-chB
	test.Equal(t, currentB[0].Type, chain.HCCurrent)
	test.Equal(t, currentB[0].Val, genTS)

	defer ctx.Done()
	assertSetHead(t, chainStore, link1)
	assertSetHead(t, chainStore, link2)
	assertSetHead(t, chainStore, link3)
	assertSetHead(t, chainStore, link4)
	assertSetHead(t, chainStore, link3)
	assertSetHead(t, chainStore, link2)
	assertSetHead(t, chainStore, link1)
	assertSetHead(t, chainStore, genTS)
	heads := []*types.TipSet{genTS, link1, link2, link3, link4, link4, link3, link2, link1, genTS}
	types := []string{"apply", "apply", "apply", "apply", "apply", "revert", "revert", "revert", "revert"}
	// Heads arrive in the expected order
	for i := 0; i < 9; i++ {
		headA := <-chA
		headB := <-chB
		assert.Equal(t, headA[0].Type, types[i])
		test.Equal(t, headA, headB)
		test.Equal(t, headA[0].Val, heads[i])
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
	genTS := builder.Genesis()
	rPriv := repo.NewInMemoryRepo()
	bs := rPriv.Datastore()
	ds := rPriv.ChainDatastore()
	cst := cbor.NewCborStore(bs)

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

	cboreStore := &CborBlockStore{
		Store:     chain.NewStore(ds, cst, bs, chain.NewStatusReporter(), config.DefaultForkUpgradeParam, genTS.At(0).Cid()),
		cborStore: cst,
	}
	requirePutTestChain(ctx, t, cboreStore, link4.Key(), builder, 5)
	assertSetHead(t, cboreStore, genTS) // set the genesis block

	assertSetHead(t, cboreStore, link4)
	cboreStore.Stop()

	// rebuild chain with same datastore and cborstore
	sr := chain.NewStatusReporter()
	rebootChain := chain.NewStore(ds, cst, bs, sr, config.DefaultForkUpgradeParam, genTS.At(0).Cid())
	rebootCbore := &CborBlockStore{
		Store: rebootChain,
	}

	err := rebootChain.Load(ctx)
	assert.NoError(t, err)

	// Check that chain store has index
	// Get a tipset and state by key
	got2 := requireGetTipSet(ctx, t, rebootCbore, link2.Key())
	assert.ObjectsAreEqualValues(link2, got2)

	// Get another by parent key
	got4 := requireSiblingState(t, rebootCbore, link4)
	assert.Equal(t, 1, len(got4))
	assert.ObjectsAreEqualValues(link4, got4[0].TipSet)

	// Check the head
	test.Equal(t, link4, rebootChain.GetHead())
}

func requireGetTipSet(ctx context.Context, t *testing.T, chainStore *CborBlockStore, key types.TipSetKey) *types.TipSet {
	ts, err := chainStore.GetTipSet(key)
	require.NoError(t, err)
	return ts
}

type tipSetStateRootGetter interface {
	GetTipSetStateRoot(*types.TipSet) (cid.Cid, error)
}

func requireGetTipSetStateRoot(ctx context.Context, t *testing.T, chainStore tipSetStateRootGetter, ts *types.TipSet) cid.Cid {
	stateCid, err := chainStore.GetTipSetStateRoot(ts)
	require.NoError(t, err)
	return stateCid
}
