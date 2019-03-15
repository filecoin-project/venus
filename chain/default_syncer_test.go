package chain_test

import (
	"context"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	bstore "gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

var (
	// Chain diagram below.  Note that blocks in the same tipset are in parentheses.
	//
	// genesis -> (link1blk1, link1blk2) -> (link2blk1, link2blk2, link2blk3) -> link3blk1 -> (link4blk1, link4blk2)

	// Blocks
	genesis                         *types.Block
	link1blk1, link1blk2            *types.Block
	link2blk1, link2blk2, link2blk3 *types.Block
	link3blk1                       *types.Block
	link4blk1, link4blk2            *types.Block

	// Cids
	genCid                                                       cid.Cid
	genStateRoot, link1State, link2State, link3State, link4State cid.Cid

	// TipSets
	genTS, link1, link2, link3, link4 types.TipSet

	// utils
	cidGetter         func() cid.Cid
	minerAddress      address.Address
	minerOwnerAddress address.Address
	minerPeerID       peer.ID
)

func init() {
	var err error
	minerAddress, err = address.NewActorAddress([]byte("miner"))
	if err != nil {
		panic(err)
	}
	minerOwnerAddress, err = address.NewActorAddress([]byte("minerOwner"))
	if err != nil {
		panic(err)
	}
	minerPeerID = testhelpers.RequireRandomPeerID()

	// Set up the test chain
	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := hamt.NewCborStore()
	genesis, err = initGenesis(cst, bs)
	if err != nil {
		panic(err)
	}
	genCid = genesis.Cid()
	genTS = chain.MustNewTipSet(genesis)

	// mock state root cids
	cidGetter = types.NewCidForTestGetter()

	genStateRoot = genesis.StateRoot
}

// This function sets global variables according to the tests needs.  The
// test chain's basic structure is always the same, but some tests want
// mocked stateRoots or parent weight calculations from different consensus protocols.
func requireSetTestChain(require *require.Assertions, con consensus.Protocol, mockStateRoots bool) {

	var err error
	// see powerTableForWidenTest
	minerPower := uint64(25)
	totalPower := uint64(100)
	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := mockSigner.PubKeys[0]

	fakeChildParams := chain.FakeChildParams{
		Parent:      genTS,
		GenesisCid:  genCid,
		StateRoot:   genStateRoot,
		Consensus:   con,
		MinerAddr:   minerAddress,
		MinerPubKey: mockSignerPubKey,
		Signer:      mockSigner,
	}

	link1blk1 = chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	link1blk1.Proof, link1blk1.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, mockSigner)
	require.NoError(err)

	link1blk2 = chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	link1blk2.Proof, link1blk2.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, mockSigner)
	require.NoError(err)

	link1 = testhelpers.RequireNewTipSet(require, link1blk1, link1blk2)

	if mockStateRoots {
		link1State = cidGetter()
	} else {
		link1State = genStateRoot
	}

	fakeChildParams.Parent = link1
	fakeChildParams.StateRoot = link1State
	link2blk1 = chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	link2blk1.Proof, link2blk1.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, mockSigner)
	require.NoError(err)

	link2blk2 = chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	link2blk2.Proof, link2blk2.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, mockSigner)
	require.NoError(err)

	fakeChildParams.Nonce = uint64(1)
	link2blk3 = chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	link2blk3.Proof, link2blk3.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, mockSigner)
	require.NoError(err)

	link2 = testhelpers.RequireNewTipSet(require, link2blk1, link2blk2, link2blk3)

	if mockStateRoots {
		link2State = cidGetter()
	} else {
		link2State = genStateRoot
	}

	fakeChildParams.Parent = link2
	fakeChildParams.StateRoot = link2State
	link3blk1 = chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	link3blk1.Proof, link3blk1.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, mockSigner)
	require.NoError(err)

	link3 = testhelpers.RequireNewTipSet(require, link3blk1)

	if mockStateRoots {
		link3State = cidGetter()
	} else {
		link3State = genStateRoot
	}

	fakeChildParams.Parent = link3
	fakeChildParams.StateRoot = link3State
	fakeChildParams.NullBlockCount = uint64(2)
	link4blk1 = chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	link4blk1.Proof, link4blk1.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, mockSigner)
	require.NoError(err)

	fakeChildParams.Nonce = uint64(1)
	link4blk2 = chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	link4blk2.Proof, link4blk2.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, mockSigner)
	require.NoError(err)

	link4 = testhelpers.RequireNewTipSet(require, link4blk1, link4blk2)

	if mockStateRoots {
		link4State = cidGetter()
	} else {
		link4State = genStateRoot
	}
}

// loadSyncerFromRepo creates a store and syncer from an existing repo.
func loadSyncerFromRepo(require *require.Assertions, r repo.Repo) (*chain.DefaultSyncer, *hamt.CborIpldStore) {
	powerTable := &testhelpers.TestView{}
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	verifier := proofs.NewFakeVerifier(true, nil)
	con := consensus.NewExpected(cst, bs, testhelpers.NewTestProcessor(), powerTable, genCid, verifier)
	syncer, testchain, cst, _ := initSyncTest(require, con, initGenesis, cst, bs, r)
	ctx := context.Background()
	err := testchain.Load(ctx)
	require.NoError(err)
	return syncer, cst
}

// initSyncTestDefault creates and returns the datastructures (chain store, syncer, etc)
// needed to run tests.  It also sets the global test variables appropriately.
func initSyncTestDefault(require *require.Assertions) (*chain.DefaultSyncer, chain.Store, *hamt.CborIpldStore, repo.Repo) {
	processor := testhelpers.NewTestProcessor()
	powerTable := &testhelpers.TestView{}
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	verifier := proofs.NewFakeVerifier(true, nil)
	con := consensus.NewExpected(cst, bs, processor, powerTable, genCid, verifier)
	requireSetTestChain(require, con, false)
	return initSyncTest(require, con, initGenesis, cst, bs, r)
}

// initSyncTestWithPowerTable creates and returns the datastructures (chain store, syncer, etc)
// needed to run tests.  It also sets the global test variables appropriately.
func initSyncTestWithPowerTable(require *require.Assertions, powerTable consensus.PowerTableView) (*chain.DefaultSyncer, chain.Store, *hamt.CborIpldStore, consensus.Protocol) {
	processor := testhelpers.NewTestProcessor()
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	verifier := proofs.NewFakeVerifier(true, nil)
	con := consensus.NewExpected(cst, bs, processor, powerTable, genCid, verifier)
	requireSetTestChain(require, con, false)
	sync, testchain, cst, _ := initSyncTest(require, con, initGenesis, cst, bs, r)
	return sync, testchain, cst, con
}

func initSyncTest(require *require.Assertions, con consensus.Protocol, genFunc func(cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error), cst *hamt.CborIpldStore, bs bstore.Blockstore, r repo.Repo) (*chain.DefaultSyncer, chain.Store, *hamt.CborIpldStore, repo.Repo) {
	ctx := context.Background()

	calcGenBlk, err := genFunc(cst, bs) // flushes state
	require.NoError(err)

	calcGenBlk.StateRoot = genStateRoot

	chainDS := r.ChainDatastore()
	chainStore := chain.NewDefaultStore(chainDS, cst, calcGenBlk.Cid())

	syncer := chain.NewDefaultSyncer(cst, cst, con, chainStore) // note we use same cst for on and offline for tests

	// Initialize stores to contain genesis block and state
	calcGenTS := testhelpers.RequireNewTipSet(require, calcGenBlk)

	genTsas := &chain.TipSetAndState{
		TipSet:          calcGenTS,
		TipSetStateRoot: genStateRoot,
	}
	chain.RequirePutTsas(ctx, require, chainStore, genTsas)
	err = chainStore.SetHead(ctx, calcGenTS) // Initialize chainStore store with correct genesis
	require.NoError(err)
	requireHead(require, chainStore, calcGenTS)
	requireTsAdded(require, chainStore, calcGenTS)

	return syncer, chainStore, cst, r
}

func containsTipSet(tsasSlice []*chain.TipSetAndState, ts types.TipSet) bool {
	for _, tsas := range tsasSlice {
		if tsas.TipSet.String() == ts.String() { //bingo
			return true
		}
	}
	return false
}

func requireTsAdded(require *require.Assertions, chain chain.Store, ts types.TipSet) {
	ctx := context.Background()
	h, err := ts.Height()
	require.NoError(err)
	// Tip Index correctly updated
	gotTsas, err := chain.GetTipSetAndState(ctx, ts.String())
	require.NoError(err)
	require.Equal(ts, gotTsas.TipSet)
	parent, err := ts.Parents()
	require.NoError(err)
	childTsasSlice, err := chain.GetTipSetAndStatesByParentsAndHeight(ctx, parent.String(), h)
	require.NoError(err)
	require.True(containsTipSet(childTsasSlice, ts))

	// Blocks exist in store
	for _, blk := range ts {
		require.True(chain.HasBlock(ctx, blk.Cid()))
	}
}

func assertTsAdded(assert *assert.Assertions, chainStore chain.Store, ts types.TipSet) {
	ctx := context.Background()
	h, err := ts.Height()
	assert.NoError(err)
	// Tip Index correctly updated
	gotTsas, err := chainStore.GetTipSetAndState(ctx, ts.String())
	assert.NoError(err)
	assert.Equal(ts, gotTsas.TipSet)
	parent, err := ts.Parents()
	assert.NoError(err)
	childTsasSlice, err := chainStore.GetTipSetAndStatesByParentsAndHeight(ctx, parent.String(), h)
	assert.NoError(err)
	assert.True(containsTipSet(childTsasSlice, ts))

	// Blocks exist in store
	for _, blk := range ts {
		assert.True(chainStore.HasBlock(ctx, blk.Cid()))
	}
}

func assertNoAdd(assert *assert.Assertions, chainStore chain.Store, cids []cid.Cid) {
	ctx := context.Background()
	// Tip Index correctly updated
	_, err := chainStore.GetTipSetAndState(ctx, types.NewSortedCidSet(cids...).String())
	assert.Error(err)
	// Blocks exist in store
	for _, c := range cids {
		assert.False(chainStore.HasBlock(ctx, c))
	}
}

func requireHead(require *require.Assertions, chain chain.Store, head types.TipSet) {
	gotHead := chain.Head()
	require.Equal(head, gotHead)
}

func assertHead(assert *assert.Assertions, chain chain.Store, head types.TipSet) {
	gotHead := chain.Head()
	assert.Equal(head, gotHead)
}

func requirePutBlocks(require *require.Assertions, cst *hamt.CborIpldStore, blks ...*types.Block) []cid.Cid {
	ctx := context.Background()
	var cids []cid.Cid
	for _, blk := range blks {
		c, err := cst.Put(ctx, blk)
		require.NoError(err)
		cids = append(cids, c)
	}
	return cids
}

/* Regular Degular syncing */

// Syncer syncs a single block
func TestSyncOneBlock(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chainStore, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()
	expectedTs := testhelpers.RequireNewTipSet(require, link1blk1)

	cids := requirePutBlocks(require, cst, link1blk1)
	err := syncer.HandleNewBlocks(ctx, cids)
	assert.NoError(err)

	assertTsAdded(assert, chainStore, expectedTs)
	assertHead(assert, chainStore, expectedTs)
}

// Syncer syncs a single tipset.
func TestSyncOneTipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chainStore, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	cids := requirePutBlocks(require, cst, link1blk1, link1blk2)
	err := syncer.HandleNewBlocks(ctx, cids)
	assert.NoError(err)

	assertTsAdded(assert, chainStore, link1)
	assertHead(assert, chainStore, link1)
}

// Syncer syncs one tipset, block by block.
func TestSyncTipSetBlockByBlock(t *testing.T) {
	pt := testhelpers.NewTestPowerTableView(1, 1)
	assert := assert.New(t)
	require := require.New(t)
	syncer, chainStore, cst, _ := initSyncTestWithPowerTable(require, pt)
	ctx := context.Background()
	expTs1 := testhelpers.RequireNewTipSet(require, link1blk1)

	cids := requirePutBlocks(require, cst, link1blk1, link1blk2)
	err := syncer.HandleNewBlocks(ctx, []cid.Cid{cids[0]})
	assert.NoError(err)

	assertTsAdded(assert, chainStore, expTs1)
	assertHead(assert, chainStore, expTs1)

	err = syncer.HandleNewBlocks(ctx, []cid.Cid{cids[1]})
	assert.NoError(err)

	assertTsAdded(assert, chainStore, link1)
	assertHead(assert, chainStore, link1)
}

// Syncer syncs a chain, tipset by tipset.
func TestSyncChainTipSetByTipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chainStore, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	cids1 := requirePutBlocks(require, cst, link1.ToSlice()...)
	cids2 := requirePutBlocks(require, cst, link2.ToSlice()...)
	cids3 := requirePutBlocks(require, cst, link3.ToSlice()...)
	cids4 := requirePutBlocks(require, cst, link4.ToSlice()...)

	err := syncer.HandleNewBlocks(ctx, cids1)
	assert.NoError(err)
	assertTsAdded(assert, chainStore, link1)
	assertHead(assert, chainStore, link1)

	err = syncer.HandleNewBlocks(ctx, cids2)
	assert.NoError(err)
	assertTsAdded(assert, chainStore, link2)
	assertHead(assert, chainStore, link2)

	err = syncer.HandleNewBlocks(ctx, cids3)
	assert.NoError(err)
	assertTsAdded(assert, chainStore, link3)
	assertHead(assert, chainStore, link3)

	err = syncer.HandleNewBlocks(ctx, cids4)
	assert.NoError(err)
	assertTsAdded(assert, chainStore, link4)
	assertHead(assert, chainStore, link4)
}

// Syncer syncs a whole chain given only the head cids.
func TestSyncChainHead(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chainStore, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	_ = requirePutBlocks(require, cst, link3.ToSlice()...)
	cids4 := requirePutBlocks(require, cst, link4.ToSlice()...)

	err := syncer.HandleNewBlocks(ctx, cids4)
	assert.NoError(err)
	assertTsAdded(assert, chainStore, link4)
	assertTsAdded(assert, chainStore, link3)
	assertTsAdded(assert, chainStore, link2)
	assertTsAdded(assert, chainStore, link1)
	assertHead(assert, chainStore, link4)
}

// Syncer determines the heavier fork.
func TestSyncIgnoreLightFork(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chainStore, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	forkbase := testhelpers.RequireNewTipSet(require, link2blk1)
	signer, ki := types.NewMockSignersAndKeyInfo(1)
	signerPubKey := ki[0].PublicKey()

	forkblk1 := chain.RequireMkFakeChild(require,
		chain.FakeChildParams{
			MinerAddr:   minerAddress,
			Signer:      signer,
			MinerPubKey: signerPubKey,
			Parent:      forkbase,
			GenesisCid:  genCid,
			StateRoot:   genStateRoot,
		})
	forklink1 := testhelpers.RequireNewTipSet(require, forkblk1)

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	_ = requirePutBlocks(require, cst, link3.ToSlice()...)
	cids4 := requirePutBlocks(require, cst, link4.ToSlice()...)

	forkCids1 := requirePutBlocks(require, cst, forklink1.ToSlice()...)

	// Sync heaviest branch first.
	err := syncer.HandleNewBlocks(ctx, cids4)
	assert.NoError(err)
	assertTsAdded(assert, chainStore, link4)
	assertHead(assert, chainStore, link4)

	// lighter fork should be processed but not change head.
	assert.NoError(syncer.HandleNewBlocks(ctx, forkCids1))
	assertTsAdded(assert, chainStore, forklink1)
	assertHead(assert, chainStore, link4)
}

// Correctly sync a heavier fork
func TestHeavierFork(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chainStore, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	signer, ki := types.NewMockSignersAndKeyInfo(2)
	mockSignerPubKey := ki[0].PublicKey()

	forkbase := testhelpers.RequireNewTipSet(require, link2blk1)
	fakeChildParams := chain.FakeChildParams{
		Parent:      forkbase,
		GenesisCid:  genCid,
		StateRoot:   genStateRoot,
		MinerAddr:   minerAddress,
		Signer:      signer,
		MinerPubKey: mockSignerPubKey,
		Nonce:       uint64(1),
	}

	forklink1blk1 := chain.RequireMkFakeChild(require, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink1blk2 := chain.RequireMkFakeChild(require, fakeChildParams)

	fakeChildParams.Nonce = uint64(2)
	forklink1blk3 := chain.RequireMkFakeChild(require, fakeChildParams)

	forklink1 := testhelpers.RequireNewTipSet(require, forklink1blk1, forklink1blk2, forklink1blk3)

	fakeChildParams.Parent = forklink1
	fakeChildParams.Nonce = uint64(0)
	forklink2blk1 := chain.RequireMkFakeChild(require, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink2blk2 := chain.RequireMkFakeChild(require, fakeChildParams)

	fakeChildParams.Nonce = uint64(2)
	forklink2blk3 := chain.RequireMkFakeChild(require, fakeChildParams)
	forklink2 := testhelpers.RequireNewTipSet(require, forklink2blk1, forklink2blk2, forklink2blk3)

	fakeChildParams.Nonce = uint64(0)
	fakeChildParams.Parent = forklink2
	forklink3blk1 := chain.RequireMkFakeChild(require, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink3blk2 := chain.RequireMkFakeChild(require, fakeChildParams)
	forklink3 := testhelpers.RequireNewTipSet(require, forklink3blk1, forklink3blk2)

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	_ = requirePutBlocks(require, cst, link3.ToSlice()...)
	cids4 := requirePutBlocks(require, cst, link4.ToSlice()...)
	_ = requirePutBlocks(require, cst, forklink1.ToSlice()...)
	_ = requirePutBlocks(require, cst, forklink2.ToSlice()...)
	forkHead := requirePutBlocks(require, cst, forklink3.ToSlice()...)

	err := syncer.HandleNewBlocks(ctx, cids4)
	assert.NoError(err)
	assertTsAdded(assert, chainStore, link4)
	assertHead(assert, chainStore, link4)

	// heavier fork updates head
	err = syncer.HandleNewBlocks(ctx, forkHead)
	assert.NoError(err)
	assertTsAdded(assert, chainStore, forklink1)
	assertTsAdded(assert, chainStore, forklink2)
	assertTsAdded(assert, chainStore, forklink3)
	assertHead(assert, chainStore, forklink3)
}

// Syncer errors if blocks don't form a tipset
func TestBlocksNotATipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chainStore, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	badCids := []cid.Cid{link1blk1.Cid(), link2blk1.Cid()}
	err := syncer.HandleNewBlocks(ctx, badCids)
	assert.Error(err)
	assertNoAdd(assert, chainStore, badCids)
}

/* particularly tricky edge cases relating to subtle Expected Consensus requirements */

// Syncer is capable of recovering from a fork reorg after Load.
func TestLoadFork(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chainStore, cst, r := initSyncTestDefault(require)
	ctx := context.Background()

	// Set up chain store to have standard chain up to link2
	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	cids2 := requirePutBlocks(require, cst, link2.ToSlice()...)
	err := syncer.HandleNewBlocks(ctx, cids2)
	require.NoError(err)

	// Now sync the store with a heavier fork, forking off link1.
	forkbase := testhelpers.RequireNewTipSet(require, link2blk1)

	signer, ki := types.NewMockSignersAndKeyInfo(2)
	mockSignerPubKey := ki[0].PublicKey()

	fakeChildParams := chain.FakeChildParams{
		Parent:      forkbase,
		GenesisCid:  genCid,
		MinerAddr:   minerAddress,
		Nonce:       uint64(1),
		StateRoot:   genStateRoot,
		Signer:      signer,
		MinerPubKey: mockSignerPubKey,
	}

	forklink1blk1 := chain.RequireMkFakeChild(require, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink1blk2 := chain.RequireMkFakeChild(require, fakeChildParams)

	fakeChildParams.Nonce = uint64(2)
	forklink1blk3 := chain.RequireMkFakeChild(require, fakeChildParams)
	//chain.FakeChildParams{Parent: forkbase, GenesisCid: genCid, StateRoot: genStateRoot, Nonce: uint64(2)})
	forklink1 := testhelpers.RequireNewTipSet(require, forklink1blk1, forklink1blk2, forklink1blk3)

	fakeChildParams.Parent = forklink1
	fakeChildParams.Nonce = uint64(0)
	forklink2blk1 := chain.RequireMkFakeChild(require, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink2blk2 := chain.RequireMkFakeChild(require, fakeChildParams)

	fakeChildParams.Nonce = uint64(2)
	forklink2blk3 := chain.RequireMkFakeChild(require, fakeChildParams)
	forklink2 := testhelpers.RequireNewTipSet(require, forklink2blk1, forklink2blk2, forklink2blk3)

	fakeChildParams.Nonce = uint64(0)
	fakeChildParams.Parent = forklink2
	forklink3blk1 := chain.RequireMkFakeChild(require, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink3blk2 := chain.RequireMkFakeChild(require, fakeChildParams)
	forklink3 := testhelpers.RequireNewTipSet(require, forklink3blk1, forklink3blk2)

	_ = requirePutBlocks(require, cst, forklink1.ToSlice()...)
	_ = requirePutBlocks(require, cst, forklink2.ToSlice()...)
	forkHead := requirePutBlocks(require, cst, forklink3.ToSlice()...)
	err = syncer.HandleNewBlocks(ctx, forkHead)
	require.NoError(err)
	requireHead(require, chainStore, forklink3)

	// Shut down store, reload and wire to syncer.
	loadSyncer, loadCst := loadSyncerFromRepo(require, r)

	// Test that the syncer can sync a block on the old chain
	cids3 := requirePutBlocks(require, loadCst, link3.ToSlice()...)
	err = loadSyncer.HandleNewBlocks(ctx, cids3)
	assert.NoError(err)
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
	assert := assert.New(t)
	require := require.New(t)
	syncer, _, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	// Set up store to have standard chain up to link2
	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	cids2 := requirePutBlocks(require, cst, link2.ToSlice()...)
	err := syncer.HandleNewBlocks(ctx, cids2)
	require.NoError(err)

	// Sync one tipset with a parent equal to a subset of an existing
	// tipset in the store.
	forkbase := testhelpers.RequireNewTipSet(require, link2blk1, link2blk2)

	signer, ki := types.NewMockSignersAndKeyInfo(2)
	mockSignerPubKey := ki[0].PublicKey()

	fakeChildParams := chain.FakeChildParams{
		Parent:      forkbase,
		GenesisCid:  genCid,
		MinerAddr:   minerAddress,
		StateRoot:   genStateRoot,
		Signer:      signer,
		MinerPubKey: mockSignerPubKey,
	}

	forkblk1 := chain.RequireMkFakeChild(require, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forkblk2 := chain.RequireMkFakeChild(require, fakeChildParams)

	forklink := testhelpers.RequireNewTipSet(require, forkblk1, forkblk2)
	forkHead := requirePutBlocks(require, cst, forklink.ToSlice()...)
	err = syncer.HandleNewBlocks(ctx, forkHead)
	assert.NoError(err)

	// Sync another tipset with a parent equal to a subset of the tipset
	// just synced.
	newForkbase := testhelpers.RequireNewTipSet(require, forkblk1, forkblk2)

	fakeChildParams.Parent = newForkbase
	fakeChildParams.Nonce = uint64(0)
	newForkblk := chain.RequireMkFakeChild(require, fakeChildParams)
	newForklink := testhelpers.RequireNewTipSet(require, newForkblk)
	newForkHead := requirePutBlocks(require, cst, newForklink.ToSlice()...)
	err = syncer.HandleNewBlocks(ctx, newForkHead)
	assert.NoError(err)
}

// Check that the syncer correctly adds widened chain ancestors to the store.
func TestWidenChainAncestor(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chainStore, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	signer, ki := types.NewMockSignersAndKeyInfo(2)
	mockSignerPubKey := ki[0].PublicKey()

	fakeChildParams := chain.FakeChildParams{
		MinerAddr:   minerAddress,
		Parent:      link1,
		GenesisCid:  genCid,
		StateRoot:   genStateRoot,
		Signer:      signer,
		MinerPubKey: mockSignerPubKey,
		Nonce:       uint64(27),
	}

	link2blkother := chain.RequireMkFakeChild(require, fakeChildParams)

	link2intersect := testhelpers.RequireNewTipSet(require, link2blk1, link2blkother)

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	_ = requirePutBlocks(require, cst, link3.ToSlice()...)
	cids4 := requirePutBlocks(require, cst, link4.ToSlice()...)

	intersectCids := requirePutBlocks(require, cst, link2intersect.ToSlice()...)

	// Sync the subset of link2 first
	err := syncer.HandleNewBlocks(ctx, intersectCids)
	assert.NoError(err)
	assertTsAdded(assert, chainStore, link2intersect)
	assertHead(assert, chainStore, link2intersect)

	// Sync chain with head at link4
	err = syncer.HandleNewBlocks(ctx, cids4)
	assert.NoError(err)
	assertTsAdded(assert, chainStore, link4)
	assertHead(assert, chainStore, link4)

	// Check that the widened tipset (link2intersect U link2) is tracked
	link2Union := testhelpers.RequireNewTipSet(require, link2blk1, link2blk2, link2blk3, link2blkother)
	assertTsAdded(assert, chainStore, link2Union)
}

type powerTableForWidenTest struct{}

func (pt *powerTableForWidenTest) Total(ctx context.Context, st state.Tree, bs bstore.Blockstore) (uint64, error) {
	return uint64(100), nil
}

func (pt *powerTableForWidenTest) Miner(ctx context.Context, st state.Tree, bs bstore.Blockstore, mAddr address.Address) (uint64, error) {
	return uint64(25), nil
}

func (pt *powerTableForWidenTest) HasPower(ctx context.Context, st state.Tree, bs bstore.Blockstore, mAddr address.Address) bool {
	return true
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
// Using the provided powertable all new tipsets contribute to the weight: + 35*(num of blocks in tipset).
// So, the weight of the  head of the test chain =
//   W(link1) + 105 + 35 + 70 = W(link1) + 210 = 280
// and the weight of the head of the fork chain =
//   W(link1) + 140 + 35 = W(link1) + 175 = 245
// and the weight of the union of link2 of both branches (a valid tipset) is
//   W(link1) + 245 = 315
//
// Therefore the syncer should set the head of the store to the union of the links..
func TestHeaviestIsWidenedAncestor(t *testing.T) {
	pt := &powerTableForWidenTest{}
	assert := assert.New(t)
	require := require.New(t)
	syncer, chainStore, cst, con := initSyncTestWithPowerTable(require, pt)
	ctx := context.Background()

	minerPower := uint64(25)
	totalPower := uint64(100)
	signer, ki := types.NewMockSignersAndKeyInfo(2)
	mockSignerPubKey := ki[0].PublicKey()

	fakeChildParams := chain.FakeChildParams{
		Parent:     link1,
		Consensus:  con,
		GenesisCid: genCid,
		StateRoot:  genStateRoot,
		MinerAddr:  minerAddress,
		Signer:     signer,
		Nonce:      uint64(1),
	}

	var err error
	forklink2blk1 := chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	forklink2blk1.Proof, forklink2blk1.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, signer)
	require.NoError(err)

	fakeChildParams.Nonce = uint64(52)
	forklink2blk2 := chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	forklink2blk2.Proof, forklink2blk2.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, signer)
	require.NoError(err)

	fakeChildParams.Nonce = uint64(53)
	forklink2blk3 := chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	forklink2blk3.Proof, forklink2blk3.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, signer)
	require.NoError(err)

	fakeChildParams.Nonce = uint64(54)
	forklink2blk4 := chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	forklink2blk4.Proof, forklink2blk4.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, signer)
	require.NoError(err)

	forklink2 := testhelpers.RequireNewTipSet(require, forklink2blk1, forklink2blk2, forklink2blk3, forklink2blk4)

	fakeChildParams.Nonce = uint64(0)
	fakeChildParams.Parent = forklink2
	forklink3blk1 := chain.RequireMkFakeChildWithCon(require, fakeChildParams)
	forklink3blk1.Proof, forklink3blk1.Ticket, err = chain.MakeProofAndWinningTicket(mockSignerPubKey, minerPower, totalPower, signer)
	require.NoError(err)

	forklink3 := testhelpers.RequireNewTipSet(require, forklink3blk1)

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	_ = requirePutBlocks(require, cst, link3.ToSlice()...)
	testhead := requirePutBlocks(require, cst, link4.ToSlice()...)

	_ = requirePutBlocks(require, cst, forklink2.ToSlice()...)
	forkhead := requirePutBlocks(require, cst, forklink3.ToSlice()...)

	// Put testhead
	err = syncer.HandleNewBlocks(ctx, testhead)
	assert.NoError(err)

	// Put forkhead
	err = syncer.HandleNewBlocks(ctx, forkhead)
	assert.NoError(err)

	// Assert that widened chain is the new head
	wideTs := testhelpers.RequireNewTipSet(require, link2blk1, link2blk2, link2blk3, forklink2blk1, forklink2blk2, forklink2blk3, forklink2blk4)
	assertTsAdded(assert, chainStore, wideTs)
	assertHead(assert, chainStore, wideTs)
}

/* Tests with Unmocked state */

// Syncer handles MarketView weight comparisons.
// Current issue: when creating miner mining with addr0, addr0's storage head isn't found in the blockstore
// and I can't figure out why because we pass in the correct blockstore to createminerwithpower.

func TestTipSetWeightDeep(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()

	ctx := context.Background()

	mockSigner, ki := types.NewMockSignersAndKeyInfo(3)
	signerPubKey1 := ki[0].PublicKey()
	signerPubKey2 := ki[1].PublicKey()

	// set up genesis block with power
	genCfg := &gengen.GenesisCfg{
		Keys: 4,
		Miners: []gengen.Miner{
			{
				Power: uint64(0),
			},
			{
				Power: uint64(10),
			},
			{
				Power: uint64(10),
			},
			{
				Power: uint64(980),
			},
		},
	}

	info, err := gengen.GenGen(ctx, genCfg, cst, bs, 0)
	require.NoError(err)

	var calcGenBlk types.Block
	require.NoError(cst.Get(ctx, info.GenesisCid, &calcGenBlk))

	chainStore := chain.NewDefaultStore(r.ChainDatastore(), cst, calcGenBlk.Cid())

	verifier := proofs.NewFakeVerifier(true, nil)
	con := consensus.NewExpected(cst, bs, testhelpers.NewTestProcessor(), &testhelpers.TestView{}, calcGenBlk.Cid(), verifier)

	// Initialize stores to contain genesis block and state
	calcGenTS := testhelpers.RequireNewTipSet(require, &calcGenBlk)
	genTsas := &chain.TipSetAndState{
		TipSet:          calcGenTS,
		TipSetStateRoot: calcGenBlk.StateRoot,
	}
	chain.RequirePutTsas(ctx, require, chainStore, genTsas)
	err = chainStore.SetHead(ctx, calcGenTS) // Initialize chainStore with correct genesis
	require.NoError(err)
	requireHead(require, chainStore, calcGenTS)
	requireTsAdded(require, chainStore, calcGenTS)

	// Now sync the chainStore with consensus using a MarketView.
	verifier = proofs.NewFakeVerifier(true, nil)
	con = consensus.NewExpected(cst, bs, testhelpers.NewTestProcessor(), &consensus.MarketView{}, calcGenBlk.Cid(), verifier)
	syncer := chain.NewDefaultSyncer(cst, cst, con, chainStore)
	baseTS := chainStore.Head() // this is the last block of the bootstrapping chain creating miners
	require.Equal(1, len(baseTS))
	bootstrapStateRoot := baseTS.ToSlice()[0].StateRoot
	pSt, err := state.LoadStateTree(ctx, cst, baseTS.ToSlice()[0].StateRoot, builtin.Actors)
	require.NoError(err)
	/* Test chain diagram and weight calcs */
	// (Note f1b1 = fork 1 block 1)
	//
	// f1b1 -> {f1b2a, f1b2b}
	//
	// f2b1 -> f2b2
	//
	//  sw=starting weight, apw=added parent weight, mw=miner weight, ew=expected weight
	//  w({blk})          = sw + apw + mw      = sw + ew
	//  w({fXb1})         = sw + 0   + 11      = sw + 11
	//  w({f1b1, f2b1})   = sw + 0   + 11 * 2  = sw + 22
	//  w({f1b2a, f1b2b}) = sw + 11  + 11 * 2  = sw + 33
	//  w({f2b2})         = sw + 11  + 108 	   = sw + 119
	startingWeight, err := con.Weight(ctx, baseTS, pSt)
	require.NoError(err)

	wFun := func(ts types.TipSet) (uint64, error) {
		// No power-altering messages processed from here on out.
		// And so bootstrapSt correctly retrives power table for all
		// test blocks.
		return con.Weight(ctx, ts, pSt)
	}

	fakeChildParams := chain.FakeChildParams{
		Parent:     baseTS,
		GenesisCid: calcGenBlk.Cid(),
		StateRoot:  bootstrapStateRoot,
		Signer:     mockSigner,

		MinerAddr: info.Miners[1].Address,
	}

	f1b1 := chain.RequireMkFakeChildCore(require, fakeChildParams, wFun)
	f1b1.Proof, f1b1.Ticket, err = chain.MakeProofAndWinningTicket(signerPubKey1, info.Miners[1].Power, 1000, mockSigner)
	require.NoError(err)

	fakeChildParams.Nonce = uint64(1)
	fakeChildParams.MinerAddr = info.Miners[2].Address
	f2b1 := chain.RequireMkFakeChildCore(require, fakeChildParams, wFun)
	f2b1.Proof, f2b1.Ticket, err = chain.MakeProofAndWinningTicket(signerPubKey1, info.Miners[2].Power, 1000, mockSigner)
	require.NoError(err)

	tsShared := testhelpers.RequireNewTipSet(require, f1b1, f2b1)

	// Sync first tipset, should have weight 22 + starting
	sharedCids := requirePutBlocks(require, cst, f1b1, f2b1)
	err = syncer.HandleNewBlocks(ctx, sharedCids)
	require.NoError(err)
	assertHead(assert, chainStore, tsShared)
	measuredWeight, err := wFun(chainStore.Head())
	require.NoError(err)
	expectedWeight := startingWeight + uint64(22000)
	assert.Equal(expectedWeight, measuredWeight)

	// fork 1 is heavier than the old head.
	fakeChildParams = chain.FakeChildParams{
		Parent:     testhelpers.RequireNewTipSet(require, f1b1),
		GenesisCid: calcGenBlk.Cid(),
		StateRoot:  bootstrapStateRoot,
		Signer:     mockSigner,

		MinerAddr: info.Miners[1].Address,
	}
	f1b2a := chain.RequireMkFakeChildCore(require, fakeChildParams, wFun)
	f1b2a.Proof, f1b2a.Ticket, err = chain.MakeProofAndWinningTicket(signerPubKey1, info.Miners[1].Power, 1000, mockSigner)
	require.NoError(err)

	fakeChildParams.Nonce = uint64(1)

	fakeChildParams.MinerAddr = info.Miners[2].Address
	f1b2b := chain.RequireMkFakeChildCore(require, fakeChildParams, wFun)
	f1b2b.Proof, f1b2b.Ticket, err = chain.MakeProofAndWinningTicket(signerPubKey2, info.Miners[2].Power, 1000, mockSigner)
	require.NoError(err)

	f1 := testhelpers.RequireNewTipSet(require, f1b2a, f1b2b)
	f1Cids := requirePutBlocks(require, cst, f1.ToSlice()...)
	err = syncer.HandleNewBlocks(ctx, f1Cids)
	require.NoError(err)
	assertHead(assert, chainStore, f1)
	measuredWeight, err = wFun(chainStore.Head())
	require.NoError(err)
	expectedWeight = startingWeight + uint64(33000)
	assert.Equal(expectedWeight, measuredWeight)

	// fork 2 has heavier weight because of addr3's power even though there
	// are fewer blocks in the tipset than fork 1.
	fakeChildParams = chain.FakeChildParams{
		Parent:     testhelpers.RequireNewTipSet(require, f2b1),
		GenesisCid: calcGenBlk.Cid(),
		Signer:     mockSigner,

		StateRoot: bootstrapStateRoot,
		MinerAddr: info.Miners[3].Address,
	}
	f2b2 := chain.RequireMkFakeChildCore(require, fakeChildParams, wFun)
	f2b2.Proof, f2b2.Ticket, err = chain.MakeProofAndWinningTicket(signerPubKey2, info.Miners[3].Power, 1000, mockSigner)
	require.NoError(err)

	f2 := testhelpers.RequireNewTipSet(require, f2b2)
	f2Cids := requirePutBlocks(require, cst, f2.ToSlice()...)
	err = syncer.HandleNewBlocks(ctx, f2Cids)
	require.NoError(err)
	assertHead(assert, chainStore, f2)
	measuredWeight, err = wFun(chainStore.Head())
	require.NoError(err)
	expectedWeight = startingWeight + uint64(119000)
	assert.Equal(expectedWeight, measuredWeight)
}

func requireGetTsas(ctx context.Context, require *require.Assertions, chain chain.Store, key string) *chain.TipSetAndState {
	tsas, err := chain.GetTipSetAndState(ctx, key)
	require.NoError(err)
	return tsas
}

func initGenesis(cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error) {
	return consensus.MakeGenesisFunc(
		consensus.MinerActor(minerAddress, minerOwnerAddress, []byte{}, 1000, minerPeerID, types.ZeroAttoFIL),
	)(cst, bs)
}
