package chain_test

import (
	"context"
	"testing"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-offline"
	"github.com/libp2p/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type SyncerTestParams struct {
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
}

func initDSTParams() *SyncerTestParams {
	var err error
	minerAddress, err := address.NewActorAddress([]byte("miner"))
	if err != nil {
		panic(err)
	}
	minerOwnerAddress, err := address.NewActorAddress([]byte("minerOwner"))
	if err != nil {
		panic(err)
	}
	minerPeerID, err := th.RandPeerID()
	if err != nil {
		panic(err)
	}

	// Set up the test chain
	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := hamt.NewCborStore()
	genesis, err := initGenesis(minerAddress, minerOwnerAddress, minerPeerID, cst, bs)
	if err != nil {
		panic(err)
	}
	genCid := genesis.Cid()
	genTS := th.MustNewTipSet(genesis)

	// mock state root cids
	cidGetter := types.NewCidForTestGetter()

	genStateRoot := genesis.StateRoot

	return &SyncerTestParams{
		minerAddress:      minerAddress,
		minerOwnerAddress: minerOwnerAddress,
		minerPeerID:       minerPeerID,
		genesis:           genesis,
		genCid:            genCid,
		genTS:             genTS,
		cidGetter:         cidGetter,
		genStateRoot:      genStateRoot,
	}
}

// This function sets global variables according to the tests needs.  The
// test chain's basic structure is always the same, but some tests want
// mocked stateRoots or parent weight calculations from different consensus protocols.
func requireSetTestChain(t *testing.T, con consensus.Protocol, mockStateRoots bool, dstP *SyncerTestParams) {

	var err error
	// see powerTableForWidenTest
	minerPower := types.NewBytesAmount(25)
	totalPower := types.NewBytesAmount(100)
	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)
	minerWorker := mockSigner.Addresses[0]

	fakeChildParams := th.FakeChildParams{
		Parent:      dstP.genTS,
		GenesisCid:  dstP.genCid,
		StateRoot:   dstP.genStateRoot,
		Consensus:   con,
		MinerAddr:   dstP.minerAddress,
		MinerWorker: minerWorker,
		Signer:      mockSigner,
	}

	dstP.link1blk1 = th.RequireMkFakeChildWithCon(t, fakeChildParams)
	dstP.link1blk1.Proof, dstP.link1blk1.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, mockSigner)
	require.NoError(t, err)

	dstP.link1blk2 = th.RequireMkFakeChildWithCon(t, fakeChildParams)
	dstP.link1blk2.Proof, dstP.link1blk2.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, mockSigner)
	require.NoError(t, err)

	dstP.link1 = th.RequireNewTipSet(t, dstP.link1blk1, dstP.link1blk2)

	if mockStateRoots {
		dstP.link1State = dstP.cidGetter()
	} else {
		dstP.link1State = dstP.genStateRoot
	}

	fakeChildParams.Parent = dstP.link1
	fakeChildParams.StateRoot = dstP.link1State
	dstP.link2blk1 = th.RequireMkFakeChildWithCon(t, fakeChildParams)
	dstP.link2blk1.Proof, dstP.link2blk1.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, mockSigner)
	require.NoError(t, err)

	dstP.link2blk2 = th.RequireMkFakeChildWithCon(t, fakeChildParams)
	dstP.link2blk2.Proof, dstP.link2blk2.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, mockSigner)
	require.NoError(t, err)

	fakeChildParams.Nonce = uint64(1)
	dstP.link2blk3 = th.RequireMkFakeChildWithCon(t, fakeChildParams)
	dstP.link2blk3.Proof, dstP.link2blk3.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, mockSigner)
	require.NoError(t, err)

	dstP.link2 = th.RequireNewTipSet(t, dstP.link2blk1, dstP.link2blk2, dstP.link2blk3)

	if mockStateRoots {
		dstP.link2State = dstP.cidGetter()
	} else {
		dstP.link2State = dstP.genStateRoot
	}

	fakeChildParams.Parent = dstP.link2
	fakeChildParams.StateRoot = dstP.link2State
	dstP.link3blk1 = th.RequireMkFakeChildWithCon(t, fakeChildParams)
	dstP.link3blk1.Proof, dstP.link3blk1.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, mockSigner)
	require.NoError(t, err)

	dstP.link3 = th.RequireNewTipSet(t, dstP.link3blk1)

	if mockStateRoots {
		dstP.link3State = dstP.cidGetter()
	} else {
		dstP.link3State = dstP.genStateRoot
	}

	fakeChildParams.Parent = dstP.link3
	fakeChildParams.StateRoot = dstP.link3State
	fakeChildParams.NullBlockCount = uint64(2)
	dstP.link4blk1 = th.RequireMkFakeChildWithCon(t, fakeChildParams)
	dstP.link4blk1.Proof, dstP.link4blk1.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, mockSigner)
	require.NoError(t, err)

	fakeChildParams.Nonce = uint64(1)
	dstP.link4blk2 = th.RequireMkFakeChildWithCon(t, fakeChildParams)
	dstP.link4blk2.Proof, dstP.link4blk2.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, mockSigner)
	require.NoError(t, err)

	dstP.link4 = th.RequireNewTipSet(t, dstP.link4blk1, dstP.link4blk2)

	if mockStateRoots {
		dstP.link4State = dstP.cidGetter()
	} else {
		dstP.link4State = dstP.genStateRoot
	}
}

// loadSyncerFromRepo creates a store and syncer from an existing repo.
func loadSyncerFromRepo(t *testing.T, r repo.Repo, dstP *SyncerTestParams) (*chain.Syncer, *th.TestFetcher) {
	powerTable := &th.TestView{}
	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	verifier := verification.NewFakeVerifier(true, nil)
	con := consensus.NewExpected(cst, bs, th.NewTestProcessor(), th.NewFakeBlockValidator(), powerTable, dstP.genCid, verifier, th.BlockTimeTest)

	calcGenBlk, err := initGenesis(dstP.minerAddress, dstP.minerOwnerAddress, dstP.minerPeerID, cst, bs) // flushes state
	require.NoError(t, err)
	calcGenBlk.StateRoot = dstP.genStateRoot
	chainDS := r.ChainDatastore()
	chainStore := chain.NewStore(chainDS, cst, &state.TreeStateLoader{}, calcGenBlk.Cid())

	blockSource := th.NewTestFetcher()
	syncer := chain.NewSyncer(con, chainStore, blockSource, chain.Syncing)

	ctx := context.Background()
	err = chainStore.Load(ctx)
	require.NoError(t, err)
	return syncer, blockSource
}

// initSyncTestDefault creates and returns the datastructures (syncer, store, repo, fetcher)
// needed to run tests.  It also sets the global test variables appropriately.
func initSyncTestDefault(t *testing.T, dstP *SyncerTestParams) (*chain.Syncer, *chain.Store, repo.Repo, *th.TestFetcher) {
	processor := th.NewTestProcessor()
	powerTable := &th.TestView{}
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	verifier := verification.NewFakeVerifier(true, nil)
	con := consensus.NewExpected(cst, bs, processor, th.NewFakeBlockValidator(), powerTable, dstP.genCid, verifier, th.BlockTimeTest)
	requireSetTestChain(t, con, false, dstP)
	initGenesisWrapper := func(cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error) {
		return initGenesis(dstP.minerAddress, dstP.minerOwnerAddress, dstP.minerPeerID, cst, bs)
	}
	return initSyncTest(t, con, initGenesisWrapper, cst, bs, r, dstP, chain.Syncing)
}

// initSyncTestWithMode creates and returns the datastructures (syncer, store, repo
// fetcher) needed to run tests. It also mutates the chain syncer to
// use the specified sync mode for easier testing of caught up and syncing mode
// behavior.
func initSyncTestWithMode(t *testing.T, dstP *SyncerTestParams, syncMode chain.SyncMode) (consensus.Protocol, *chain.Syncer, *th.TestFetcher) {
	processor := th.NewTestProcessor()
	powerTable := &th.TestView{}
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	verifier := verification.NewFakeVerifier(true, nil)
	con := consensus.NewExpected(cst, bs, processor, th.NewFakeBlockValidator(), powerTable, dstP.genCid, verifier, th.BlockTimeTest)
	requireSetTestChain(t, con, false, dstP)
	initGenesisWrapper := func(cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error) {
		return initGenesis(dstP.minerAddress, dstP.minerOwnerAddress, dstP.minerPeerID, cst, bs)
	}
	sync, _, _, tf := initSyncTest(t, con, initGenesisWrapper, cst, bs, r, dstP, syncMode)
	return con, sync, tf
}

// initSyncTestWithPowerTable creates and returns the datastructures (syncer, store, repo, fetcher)
// needed to run tests.  It also sets the global test variables appropriately.
func initSyncTestWithPowerTable(t *testing.T, powerTable consensus.PowerTableView, dstP *SyncerTestParams) (*chain.Syncer, *chain.Store, consensus.Protocol, *th.TestFetcher) {
	processor := th.NewTestProcessor()
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	verifier := verification.NewFakeVerifier(true, nil)
	con := consensus.NewExpected(cst, bs, processor, th.NewFakeBlockValidator(), powerTable, dstP.genCid, verifier, th.BlockTimeTest)
	requireSetTestChain(t, con, false, dstP)
	initGenesisWrapper := func(cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error) {
		return initGenesis(dstP.minerAddress, dstP.minerOwnerAddress, dstP.minerPeerID, cst, bs)
	}
	sync, testchain, _, fetcher := initSyncTest(t, con, initGenesisWrapper, cst, bs, r, dstP, chain.Syncing)
	return sync, testchain, con, fetcher
}

func initSyncTest(t *testing.T, con consensus.Protocol, genFunc func(cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error), cst *hamt.CborIpldStore, bs bstore.Blockstore, r repo.Repo, dstP *SyncerTestParams, syncMode chain.SyncMode) (*chain.Syncer, *chain.Store, repo.Repo, *th.TestFetcher) {
	ctx := context.Background()

	calcGenBlk, err := genFunc(cst, bs) // flushes state
	require.NoError(t, err)
	calcGenBlk.StateRoot = dstP.genStateRoot
	chainDS := r.ChainDatastore()
	chainStore := chain.NewStore(chainDS, cst, &state.TreeStateLoader{}, calcGenBlk.Cid())

	fetcher := th.NewTestFetcher()
	syncer := chain.NewSyncer(con, chainStore, fetcher, syncMode) // note we use same cst for on and offline for tests

	// Initialize stores to contain dstP.genesis block and state
	calcGenTS := th.RequireNewTipSet(t, calcGenBlk)

	genTsas := &chain.TipSetAndState{
		TipSet:          calcGenTS,
		TipSetStateRoot: dstP.genStateRoot,
	}
	require.NoError(t, chainStore.PutTipSetAndState(ctx, genTsas))
	err = chainStore.SetHead(ctx, calcGenTS) // Initialize chainStore store with correct dstP.genesis
	require.NoError(t, err)
	requireHead(t, chainStore, calcGenTS)
	requireTsAdded(t, chainStore, calcGenTS)

	return syncer, chainStore, r, fetcher
}

func containsTipSet(tsasSlice []*chain.TipSetAndState, ts types.TipSet) bool {
	for _, tsas := range tsasSlice {
		if tsas.TipSet.String() == ts.String() { //bingo
			return true
		}
	}
	return false
}

type requireTsAddedChainStore interface {
	GetTipSet(types.TipSetKey) (types.TipSet, error)
	GetTipSetAndStatesByParentsAndHeight(string, uint64) ([]*chain.TipSetAndState, error)
}

func requireTsAdded(t *testing.T, chain requireTsAddedChainStore, ts types.TipSet) {
	h, err := ts.Height()
	require.NoError(t, err)
	// Tip Index correctly updated
	gotTs, err := chain.GetTipSet(ts.Key())
	require.NoError(t, err)
	require.Equal(t, ts, gotTs)
	parent, err := ts.Parents()
	require.NoError(t, err)
	childTsasSlice, err := chain.GetTipSetAndStatesByParentsAndHeight(parent.String(), h)
	require.NoError(t, err)
	require.True(t, containsTipSet(childTsasSlice, ts))
}

func assertTsAdded(t *testing.T, chainStore requireTsAddedChainStore, ts types.TipSet) {
	h, err := ts.Height()
	assert.NoError(t, err)
	// Tip Index correctly updated
	gotTs, err := chainStore.GetTipSet(ts.Key())
	assert.NoError(t, err)
	assert.Equal(t, ts, gotTs)
	parent, err := ts.Parents()
	assert.NoError(t, err)
	childTsasSlice, err := chainStore.GetTipSetAndStatesByParentsAndHeight(parent.String(), h)
	assert.NoError(t, err)
	assert.True(t, containsTipSet(childTsasSlice, ts))
}

func assertNoAdd(t *testing.T, chainStore requireTsAddedChainStore, cids types.TipSetKey) {
	// Tip Index correctly updated
	_, err := chainStore.GetTipSet(cids)
	assert.Error(t, err)
}

func requireHead(t *testing.T, chain HeadAndTipsetGetter, head types.TipSet) {
	require.Equal(t, head, requireHeadTipset(t, chain))
}

func assertHead(t *testing.T, chain HeadAndTipsetGetter, head types.TipSet) {
	headTipSet, err := chain.GetTipSet(chain.GetHead())
	assert.NoError(t, err)
	assert.Equal(t, head, headTipSet)
}

func requirePutBlocks(_ *testing.T, f *th.TestFetcher, blocks ...*types.Block) types.TipSetKey {
	var cids []cid.Cid
	for _, block := range blocks {
		c := block.Cid()
		cids = append(cids, c)
	}
	f.AddSourceBlocks(blocks...)
	return types.NewTipSetKey(cids...)
}

/* Regular Degular syncing */

// Syncer syncs a single block
func TestSyncOneBlock(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	syncer, chainStore, _, blockSource := initSyncTestDefault(t, dstP)
	ctx := context.Background()
	expectedTs := th.RequireNewTipSet(t, dstP.link1blk1)

	cids := requirePutBlocks(t, blockSource, dstP.link1blk1)
	err := syncer.HandleNewTipset(ctx, cids)
	assert.NoError(t, err)

	assertTsAdded(t, chainStore, expectedTs)
	assertHead(t, chainStore, expectedTs)
}

// Syncer syncs a single tipset.
func TestSyncOneTipSet(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	syncer, chainStore, _, blockSource := initSyncTestDefault(t, dstP)
	ctx := context.Background()

	cids := requirePutBlocks(t, blockSource, dstP.link1blk1, dstP.link1blk2)
	err := syncer.HandleNewTipset(ctx, cids)
	assert.NoError(t, err)

	assertTsAdded(t, chainStore, dstP.link1)
	assertHead(t, chainStore, dstP.link1)
}

// Syncer syncs one tipset, block by block.
func TestSyncTipSetBlockByBlock(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	pt := th.NewTestPowerTableView(types.NewBytesAmount(1), types.NewBytesAmount(1))
	syncer, chainStore, _, blockSource := initSyncTestWithPowerTable(t, pt, dstP)
	ctx := context.Background()
	expTs1 := th.RequireNewTipSet(t, dstP.link1blk1)

	_ = requirePutBlocks(t, blockSource, dstP.link1blk1, dstP.link1blk2)
	err := syncer.HandleNewTipset(ctx, types.NewTipSetKey(dstP.link1blk1.Cid()))
	assert.NoError(t, err)

	assertTsAdded(t, chainStore, expTs1)
	assertHead(t, chainStore, expTs1)

	err = syncer.HandleNewTipset(ctx, types.NewTipSetKey(dstP.link1blk2.Cid()))
	assert.NoError(t, err)

	assertTsAdded(t, chainStore, dstP.link1)
	assertHead(t, chainStore, dstP.link1)
}

// Syncer syncs a chain, tipset by tipset.
func TestSyncChainTipSetByTipSet(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	syncer, chainStore, _, blockSource := initSyncTestDefault(t, dstP)
	ctx := context.Background()

	cids1 := requirePutBlocks(t, blockSource, dstP.link1.ToSlice()...)
	cids2 := requirePutBlocks(t, blockSource, dstP.link2.ToSlice()...)
	cids3 := requirePutBlocks(t, blockSource, dstP.link3.ToSlice()...)
	cids4 := requirePutBlocks(t, blockSource, dstP.link4.ToSlice()...)

	err := syncer.HandleNewTipset(ctx, cids1)
	assert.NoError(t, err)
	assertTsAdded(t, chainStore, dstP.link1)
	assertHead(t, chainStore, dstP.link1)

	err = syncer.HandleNewTipset(ctx, cids2)
	assert.NoError(t, err)
	assertTsAdded(t, chainStore, dstP.link2)
	assertHead(t, chainStore, dstP.link2)

	err = syncer.HandleNewTipset(ctx, cids3)
	assert.NoError(t, err)
	assertTsAdded(t, chainStore, dstP.link3)
	assertHead(t, chainStore, dstP.link3)

	err = syncer.HandleNewTipset(ctx, cids4)
	assert.NoError(t, err)
	assertTsAdded(t, chainStore, dstP.link4)
	assertHead(t, chainStore, dstP.link4)
}

// Syncer syncs a whole chain given only the head cids.
func TestSyncChainHead(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	syncer, chainStore, _, blockSource := initSyncTestDefault(t, dstP)
	ctx := context.Background()

	_ = requirePutBlocks(t, blockSource, dstP.link1.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, dstP.link2.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, dstP.link3.ToSlice()...)
	cids4 := requirePutBlocks(t, blockSource, dstP.link4.ToSlice()...)

	err := syncer.HandleNewTipset(ctx, cids4)
	assert.NoError(t, err)
	assertTsAdded(t, chainStore, dstP.link4)
	assertTsAdded(t, chainStore, dstP.link3)
	assertTsAdded(t, chainStore, dstP.link2)
	assertTsAdded(t, chainStore, dstP.link1)
	assertHead(t, chainStore, dstP.link4)
}

// Syncer determines the heavier fork.
func TestSyncIgnoreLightFork(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	syncer, chainStore, _, blockSource := initSyncTestDefault(t, dstP)
	ctx := context.Background()

	forkbase := th.RequireNewTipSet(t, dstP.link2blk1)
	signer, ki := types.NewMockSignersAndKeyInfo(1)
	minerWorker, err := ki[0].Address()
	require.NoError(t, err)

	forkblk1 := th.RequireMkFakeChild(t,
		th.FakeChildParams{
			MinerAddr:   dstP.minerAddress,
			Signer:      signer,
			MinerWorker: minerWorker,
			Parent:      forkbase,
			GenesisCid:  dstP.genCid,
			StateRoot:   dstP.genStateRoot,
		})
	forklink1 := th.RequireNewTipSet(t, forkblk1)

	_ = requirePutBlocks(t, blockSource, dstP.link1.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, dstP.link2.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, dstP.link3.ToSlice()...)
	cids4 := requirePutBlocks(t, blockSource, dstP.link4.ToSlice()...)

	forkCids1 := requirePutBlocks(t, blockSource, forklink1.ToSlice()...)

	// Sync heaviest branch first.
	err = syncer.HandleNewTipset(ctx, cids4)
	assert.NoError(t, err)
	assertTsAdded(t, chainStore, dstP.link4)
	assertHead(t, chainStore, dstP.link4)

	// lighter fork should be processed but not change head.
	assert.NoError(t, syncer.HandleNewTipset(ctx, forkCids1))
	assertTsAdded(t, chainStore, forklink1)
	assertHead(t, chainStore, dstP.link4)
}

// Correctly sync a heavier fork
func TestHeavierFork(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	syncer, chainStore, _, blockSource := initSyncTestDefault(t, dstP)
	ctx := context.Background()

	signer, ki := types.NewMockSignersAndKeyInfo(2)
	minerWorker, err := ki[0].Address()
	require.NoError(t, err)

	forkbase := th.RequireNewTipSet(t, dstP.link2blk1)
	fakeChildParams := th.FakeChildParams{
		Parent:      forkbase,
		GenesisCid:  dstP.genCid,
		StateRoot:   dstP.genStateRoot,
		MinerAddr:   dstP.minerAddress,
		Signer:      signer,
		MinerWorker: minerWorker,
		Nonce:       uint64(1),
	}

	forklink1blk1 := th.RequireMkFakeChild(t, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink1blk2 := th.RequireMkFakeChild(t, fakeChildParams)

	fakeChildParams.Nonce = uint64(2)
	forklink1blk3 := th.RequireMkFakeChild(t, fakeChildParams)

	forklink1 := th.RequireNewTipSet(t, forklink1blk1, forklink1blk2, forklink1blk3)

	fakeChildParams.Parent = forklink1
	fakeChildParams.Nonce = uint64(0)
	forklink2blk1 := th.RequireMkFakeChild(t, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink2blk2 := th.RequireMkFakeChild(t, fakeChildParams)

	fakeChildParams.Nonce = uint64(2)
	forklink2blk3 := th.RequireMkFakeChild(t, fakeChildParams)
	forklink2 := th.RequireNewTipSet(t, forklink2blk1, forklink2blk2, forklink2blk3)

	fakeChildParams.Nonce = uint64(0)
	fakeChildParams.Parent = forklink2
	forklink3blk1 := th.RequireMkFakeChild(t, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink3blk2 := th.RequireMkFakeChild(t, fakeChildParams)
	forklink3 := th.RequireNewTipSet(t, forklink3blk1, forklink3blk2)

	_ = requirePutBlocks(t, blockSource, dstP.link1.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, dstP.link2.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, dstP.link3.ToSlice()...)
	cids4 := requirePutBlocks(t, blockSource, dstP.link4.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, forklink1.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, forklink2.ToSlice()...)
	forkHead := requirePutBlocks(t, blockSource, forklink3.ToSlice()...)

	err = syncer.HandleNewTipset(ctx, cids4)
	assert.NoError(t, err)
	assertTsAdded(t, chainStore, dstP.link4)
	assertHead(t, chainStore, dstP.link4)

	// heavier fork updates head
	err = syncer.HandleNewTipset(ctx, forkHead)
	assert.NoError(t, err)
	assertTsAdded(t, chainStore, forklink1)
	assertTsAdded(t, chainStore, forklink2)
	assertTsAdded(t, chainStore, forklink3)
	assertHead(t, chainStore, forklink3)
}

// Syncer errors when input blocks massively exceed the current block height in
// caught up mode
func TestFarFutureTipsetsWhenCaughtUp(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)
	dstP := initDSTParams()
	con, syncer, blockSource := initSyncTestWithMode(t, dstP, chain.CaughtUp)
	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)
	minerWorker := mockSigner.Addresses[0]
	fakeChildParams := th.FakeChildParams{
		Parent:      dstP.genTS,
		GenesisCid:  dstP.genCid,
		StateRoot:   dstP.genStateRoot,
		Consensus:   con,
		MinerAddr:   dstP.minerAddress,
		MinerWorker: minerWorker,
		Signer:      mockSigner,
	}
	ctx := context.Background()
	minerPower := types.NewBytesAmount(25)
	totalPower := types.NewBytesAmount(100)

	var err error
	var tipsetCids types.TipSetKey
	for i := 0; i < chain.FinalityLimit+10; i++ {
		require.NoError(t, err)

		linkBlk := th.RequireMkFakeChildWithCon(t, fakeChildParams)
		linkBlk.Proof, linkBlk.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, mockSigner)
		require.NoError(t, err)

		fakeChildParams.Parent = th.RequireNewTipSet(t, linkBlk)
		tipsetCids = requirePutBlocks(t, blockSource, linkBlk)
	}

	assert.Error(t, syncer.HandleNewTipset(ctx, tipsetCids))
}

// Syncer succeeds when input blocks massively exceed the current block height
// in syncing mode
func TestFarFutureTipsetsWhenSyncing(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)
	dstP := initDSTParams()
	con, syncer, blockSource := initSyncTestWithMode(t, dstP, chain.Syncing)
	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)
	minerWorker := mockSigner.Addresses[0]
	fakeChildParams := th.FakeChildParams{
		Parent:      dstP.genTS,
		GenesisCid:  dstP.genCid,
		StateRoot:   dstP.genStateRoot,
		Consensus:   con,
		MinerAddr:   dstP.minerAddress,
		MinerWorker: minerWorker,
		Signer:      mockSigner,
	}
	ctx := context.Background()
	minerPower := types.NewBytesAmount(25)
	totalPower := types.NewBytesAmount(100)

	var err error
	var tipsetCids types.TipSetKey
	for i := 0; i < chain.FinalityLimit+1; i++ {
		require.NoError(t, err)

		linkBlk := th.RequireMkFakeChildWithCon(t, fakeChildParams)
		linkBlk.Proof, linkBlk.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, mockSigner)
		require.NoError(t, err)

		fakeChildParams.Parent = th.RequireNewTipSet(t, linkBlk)
		tipsetCids = requirePutBlocks(t, blockSource, linkBlk)
	}

	assert.NoError(t, syncer.HandleNewTipset(ctx, tipsetCids))
}

// Syncer errors if blocks don't form a tipset
func TestBlocksNotATipSet(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	syncer, chainStore, _, blockSource := initSyncTestDefault(t, dstP)
	ctx := context.Background()

	_ = requirePutBlocks(t, blockSource, dstP.link1.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, dstP.link2.ToSlice()...)
	badCids := types.NewTipSetKey(dstP.link1blk1.Cid(), dstP.link2blk1.Cid())
	err := syncer.HandleNewTipset(ctx, badCids)
	assert.Error(t, err)
	assertNoAdd(t, chainStore, badCids)
}

/* particularly tricky edge cases relating to subtle Expected Consensus requirements */

// Syncer is capable of recovering from a fork reorg after Load.
func TestLoadFork(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	syncer, chainStore, r, blockSource := initSyncTestDefault(t, dstP)
	ctx := context.Background()

	// Set up chain store to have standard chain up to dstP.link2
	_ = requirePutBlocks(t, blockSource, dstP.link1.ToSlice()...)
	cids2 := requirePutBlocks(t, blockSource, dstP.link2.ToSlice()...)
	err := syncer.HandleNewTipset(ctx, cids2)
	require.NoError(t, err)

	// Now sync the store with a heavier fork, forking off dstP.link1.
	forkbase := th.RequireNewTipSet(t, dstP.link2blk1)

	signer, ki := types.NewMockSignersAndKeyInfo(2)
	minerWorker, err := ki[0].Address()
	require.NoError(t, err)

	fakeChildParams := th.FakeChildParams{
		Parent:      forkbase,
		GenesisCid:  dstP.genCid,
		MinerAddr:   dstP.minerAddress,
		Nonce:       uint64(1),
		StateRoot:   dstP.genStateRoot,
		Signer:      signer,
		MinerWorker: minerWorker,
	}

	forklink1blk1 := th.RequireMkFakeChild(t, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink1blk2 := th.RequireMkFakeChild(t, fakeChildParams)

	fakeChildParams.Nonce = uint64(2)
	forklink1blk3 := th.RequireMkFakeChild(t, fakeChildParams)
	forklink1 := th.RequireNewTipSet(t, forklink1blk1, forklink1blk2, forklink1blk3)

	fakeChildParams.Parent = forklink1
	fakeChildParams.Nonce = uint64(0)
	forklink2blk1 := th.RequireMkFakeChild(t, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink2blk2 := th.RequireMkFakeChild(t, fakeChildParams)

	fakeChildParams.Nonce = uint64(2)
	forklink2blk3 := th.RequireMkFakeChild(t, fakeChildParams)
	forklink2 := th.RequireNewTipSet(t, forklink2blk1, forklink2blk2, forklink2blk3)

	fakeChildParams.Nonce = uint64(0)
	fakeChildParams.Parent = forklink2
	forklink3blk1 := th.RequireMkFakeChild(t, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forklink3blk2 := th.RequireMkFakeChild(t, fakeChildParams)
	forklink3 := th.RequireNewTipSet(t, forklink3blk1, forklink3blk2)

	_ = requirePutBlocks(t, blockSource, forklink1.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, forklink2.ToSlice()...)
	forkHead := requirePutBlocks(t, blockSource, forklink3.ToSlice()...)
	err = syncer.HandleNewTipset(ctx, forkHead)
	require.NoError(t, err)
	requireHead(t, chainStore, forklink3)

	// Put blocks in global IPLD blockstore
	// TODO #2128 make this cleaner along with broad test cleanup.
	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	requirePutBlocksToCborStore(t, cst, dstP.genTS.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, dstP.link1.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, dstP.link2.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, forklink1.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, forklink2.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, forklink3.ToSlice()...)

	// Shut down store, reload and wire to syncer.
	loadSyncer, blockSource := loadSyncerFromRepo(t, r, dstP)

	// Test that the syncer can't sync a block on the old chain
	// without getting old blocks from network. i.e. the repo is trimmed
	// of non-heaviest chain blocks
	cids3 := requirePutBlocks(t, blockSource, dstP.link3.ToSlice()...)
	err = loadSyncer.HandleNewTipset(ctx, cids3)
	assert.Error(t, err)

	// Test that the syncer can sync a block on the heaviest chain
	// without getting old blocks from the network.
	fakeChildParams.Parent = forklink3
	forklink4blk1 := th.RequireMkFakeChild(t, fakeChildParams)
	forklink4 := th.RequireNewTipSet(t, forklink4blk1)
	cidsFork4 := requirePutBlocks(t, blockSource, forklink4.ToSlice()...)
	err = loadSyncer.HandleNewTipset(ctx, cidsFork4)
	assert.NoError(t, err)
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
	dstP := initDSTParams()

	syncer, _, _, blockSource := initSyncTestDefault(t, dstP)
	ctx := context.Background()

	// Set up store to have standard chain up to dstP.link2
	_ = requirePutBlocks(t, blockSource, dstP.link1.ToSlice()...)
	cids2 := requirePutBlocks(t, blockSource, dstP.link2.ToSlice()...)
	err := syncer.HandleNewTipset(ctx, cids2)
	require.NoError(t, err)

	// Sync one tipset with a parent equal to a subset of an existing
	// tipset in the store.
	forkbase := th.RequireNewTipSet(t, dstP.link2blk1, dstP.link2blk2)

	signer, ki := types.NewMockSignersAndKeyInfo(2)
	minerWorker, err := ki[0].Address()
	require.NoError(t, err)

	fakeChildParams := th.FakeChildParams{
		Parent:      forkbase,
		GenesisCid:  dstP.genCid,
		MinerAddr:   dstP.minerAddress,
		StateRoot:   dstP.genStateRoot,
		Signer:      signer,
		MinerWorker: minerWorker,
	}

	forkblk1 := th.RequireMkFakeChild(t, fakeChildParams)

	fakeChildParams.Nonce = uint64(1)
	forkblk2 := th.RequireMkFakeChild(t, fakeChildParams)

	forklink := th.RequireNewTipSet(t, forkblk1, forkblk2)
	forkHead := requirePutBlocks(t, blockSource, forklink.ToSlice()...)
	err = syncer.HandleNewTipset(ctx, forkHead)
	assert.NoError(t, err)

	// Sync another tipset with a parent equal to a subset of the tipset
	// just synced.
	newForkbase := th.RequireNewTipSet(t, forkblk1, forkblk2)

	fakeChildParams.Parent = newForkbase
	fakeChildParams.Nonce = uint64(0)
	newForkblk := th.RequireMkFakeChild(t, fakeChildParams)
	newForklink := th.RequireNewTipSet(t, newForkblk)
	newForkHead := requirePutBlocks(t, blockSource, newForklink.ToSlice()...)
	err = syncer.HandleNewTipset(ctx, newForkHead)
	assert.NoError(t, err)
}

// Check that the syncer correctly adds widened chain ancestors to the store.
func TestWidenChainAncestor(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	syncer, chainStore, _, blockSource := initSyncTestDefault(t, dstP)
	ctx := context.Background()

	signer, ki := types.NewMockSignersAndKeyInfo(2)
	minerWorker, err := ki[0].Address()
	require.NoError(t, err)

	fakeChildParams := th.FakeChildParams{
		MinerAddr:   dstP.minerAddress,
		Parent:      dstP.link1,
		GenesisCid:  dstP.genCid,
		StateRoot:   dstP.genStateRoot,
		Signer:      signer,
		MinerWorker: minerWorker,
		Nonce:       uint64(27),
	}

	link2blkother := th.RequireMkFakeChild(t, fakeChildParams)

	link2intersect := th.RequireNewTipSet(t, dstP.link2blk1, link2blkother)

	_ = requirePutBlocks(t, blockSource, dstP.link1.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, dstP.link2.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, dstP.link3.ToSlice()...)
	cids4 := requirePutBlocks(t, blockSource, dstP.link4.ToSlice()...)

	intersectCids := requirePutBlocks(t, blockSource, link2intersect.ToSlice()...)

	// Sync the subset of dstP.link2 first
	err = syncer.HandleNewTipset(ctx, intersectCids)
	assert.NoError(t, err)
	assertTsAdded(t, chainStore, link2intersect)
	assertHead(t, chainStore, link2intersect)

	// Sync chain with head at dstP.link4
	err = syncer.HandleNewTipset(ctx, cids4)
	assert.NoError(t, err)
	assertTsAdded(t, chainStore, dstP.link4)
	assertHead(t, chainStore, dstP.link4)

	// Check that the widened tipset (link2intersect U dstP.link2) is tracked
	link2Union := th.RequireNewTipSet(t, dstP.link2blk1, dstP.link2blk2, dstP.link2blk3, link2blkother)
	assertTsAdded(t, chainStore, link2Union)
}

type powerTableForWidenTest struct{}

func (pt *powerTableForWidenTest) Total(ctx context.Context, st state.Tree, bs bstore.Blockstore) (*types.BytesAmount, error) {
	return types.NewBytesAmount(100), nil
}

func (pt *powerTableForWidenTest) Miner(ctx context.Context, st state.Tree, bs bstore.Blockstore, mAddr address.Address) (*types.BytesAmount, error) {
	return types.NewBytesAmount(25), nil
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
// dstP.genesis -> (link1blk1, dstP.link1blk2) -> (link2blk1, dstP.link2blk2, dstP.link2blk3) -> dstP.link3blk1 -> (link4blk1, dstP.link4blk2)
//
// Now we introduce a disjoint fork on top of dstP.link1
// dstP.genesis -> (link1blk1, dstP.link1blk2) -> (forklink2blk1, forklink2blk2, forklink2blk3, forklink3blk4) -> forklink3blk1
//
// Using the provided powertable all new tipsets contribute to the weight: + 35*(num of blocks in tipset).
// So, the weight of the  head of the test chain =
//   W(link1) + 105 + 35 + 70 = W(link1) + 210 = 280
// and the weight of the head of the fork chain =
//   W(link1) + 140 + 35 = W(link1) + 175 = 245
// and the weight of the union of dstP.link2 of both branches (a valid tipset) is
//   W(link1) + 245 = 315
//
// Therefore the syncer should set the head of the store to the union of the links..
func TestHeaviestIsWidenedAncestor(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	pt := &powerTableForWidenTest{}
	syncer, chainStore, con, blockSource := initSyncTestWithPowerTable(t, pt, dstP)
	ctx := context.Background()

	minerPower := types.NewBytesAmount(25)
	totalPower := types.NewBytesAmount(100)
	signer, ki := types.NewMockSignersAndKeyInfo(2)
	minerWorker, err := ki[0].Address()
	require.NoError(t, err)

	fakeChildParams := th.FakeChildParams{
		Parent:      dstP.link1,
		Consensus:   con,
		GenesisCid:  dstP.genCid,
		StateRoot:   dstP.genStateRoot,
		MinerAddr:   dstP.minerAddress,
		MinerWorker: minerWorker,
		Signer:      signer,
		Nonce:       uint64(1),
	}

	forklink2blk1 := th.RequireMkFakeChildWithCon(t, fakeChildParams)
	forklink2blk1.Proof, forklink2blk1.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, signer)
	require.NoError(t, err)

	fakeChildParams.Nonce = uint64(52)
	forklink2blk2 := th.RequireMkFakeChildWithCon(t, fakeChildParams)
	forklink2blk2.Proof, forklink2blk2.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, signer)
	require.NoError(t, err)

	fakeChildParams.Nonce = uint64(53)
	forklink2blk3 := th.RequireMkFakeChildWithCon(t, fakeChildParams)
	forklink2blk3.Proof, forklink2blk3.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, signer)
	require.NoError(t, err)

	fakeChildParams.Nonce = uint64(54)
	forklink2blk4 := th.RequireMkFakeChildWithCon(t, fakeChildParams)
	forklink2blk4.Proof, forklink2blk4.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, signer)
	require.NoError(t, err)

	forklink2 := th.RequireNewTipSet(t, forklink2blk1, forklink2blk2, forklink2blk3, forklink2blk4)

	fakeChildParams.Nonce = uint64(0)
	fakeChildParams.Parent = forklink2
	forklink3blk1 := th.RequireMkFakeChildWithCon(t, fakeChildParams)
	forklink3blk1.Proof, forklink3blk1.Ticket, err = th.MakeProofAndWinningTicket(minerWorker, minerPower, totalPower, signer)
	require.NoError(t, err)

	forklink3 := th.RequireNewTipSet(t, forklink3blk1)

	_ = requirePutBlocks(t, blockSource, dstP.link1.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, dstP.link2.ToSlice()...)
	_ = requirePutBlocks(t, blockSource, dstP.link3.ToSlice()...)
	testhead := requirePutBlocks(t, blockSource, dstP.link4.ToSlice()...)

	_ = requirePutBlocks(t, blockSource, forklink2.ToSlice()...)
	forkhead := requirePutBlocks(t, blockSource, forklink3.ToSlice()...)

	// Put testhead
	err = syncer.HandleNewTipset(ctx, testhead)
	assert.NoError(t, err)

	// Put forkhead
	err = syncer.HandleNewTipset(ctx, forkhead)
	assert.NoError(t, err)

	// Assert that widened chain is the new head
	wideTs := th.RequireNewTipSet(t, dstP.link2blk1, dstP.link2blk2, dstP.link2blk3, forklink2blk1, forklink2blk2, forklink2blk3, forklink2blk4)
	assertTsAdded(t, chainStore, wideTs)
	assertHead(t, chainStore, wideTs)
}

/* Tests with Unmocked state */

// Syncer handles MarketView weight comparisons.
// Current issue: when creating miner mining with addr0, addr0's storage head isn't found in the blockstore
// and I can't figure out why because we pass in the correct blockstore to createStorageMinerWithpower.

func TestTipSetWeightDeep(t *testing.T) {
	tf.UnitTest(t)

	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}

	ctx := context.Background()

	mockSigner, ki := types.NewMockSignersAndKeyInfo(3)
	minerWorker1, err := ki[0].Address()
	require.NoError(t, err)
	minerWorker2, err := ki[1].Address()
	require.NoError(t, err)

	// set up dstP.genesis block with power
	genCfg := &gengen.GenesisCfg{
		ProofsMode: types.TestProofsMode,
		Keys:       4,
		Miners: []*gengen.CreateStorageMinerConfig{
			{
				NumCommittedSectors: 0,
				SectorSize:          types.OneKiBSectorSize.Uint64(),
			},
			{
				NumCommittedSectors: 10,
				SectorSize:          types.OneKiBSectorSize.Uint64(),
			},
			{
				NumCommittedSectors: 10,
				SectorSize:          types.OneKiBSectorSize.Uint64(),
			},
			{
				NumCommittedSectors: 980,
				SectorSize:          types.OneKiBSectorSize.Uint64(),
			},
		},
	}

	totalPower := types.NewBytesAmount(1000).Mul(types.OneKiBSectorSize)

	info, err := gengen.GenGen(ctx, genCfg, cst, bs, 0)
	require.NoError(t, err)

	var calcGenBlk types.Block
	require.NoError(t, cst.Get(ctx, info.GenesisCid, &calcGenBlk))

	chainStore := chain.NewStore(r.ChainDatastore(), cst, &state.TreeStateLoader{}, calcGenBlk.Cid())

	verifier := verification.NewFakeVerifier(true, nil)
	con := consensus.NewExpected(cst, bs, th.NewTestProcessor(), th.NewFakeBlockValidator(), &th.TestView{}, calcGenBlk.Cid(), verifier, th.BlockTimeTest)

	// Initialize stores to contain dstP.genesis block and state
	calcGenTS := th.RequireNewTipSet(t, &calcGenBlk)
	genTsas := &chain.TipSetAndState{
		TipSet:          calcGenTS,
		TipSetStateRoot: calcGenBlk.StateRoot,
	}
	require.NoError(t, chainStore.PutTipSetAndState(ctx, genTsas))
	err = chainStore.SetHead(ctx, calcGenTS) // Initialize chainStore with correct dstP.genesis
	require.NoError(t, err)
	requireHead(t, chainStore, calcGenTS)
	requireTsAdded(t, chainStore, calcGenTS)

	// Setup a fetcher for feeding blocks into the syncer.
	blockSource := th.NewTestFetcher()

	// Now sync the chainStore with consensus using a MarketView.
	verifier = verification.NewFakeVerifier(true, nil)
	con = consensus.NewExpected(cst, bs, th.NewTestProcessor(), th.NewFakeBlockValidator(), &consensus.MarketView{}, calcGenBlk.Cid(), verifier, th.BlockTimeTest)
	syncer := chain.NewSyncer(con, chainStore, blockSource, chain.Syncing)
	baseTS := requireHeadTipset(t, chainStore) // this is the last block of the bootstrapping chain creating miners
	require.Equal(t, 1, baseTS.Len())
	bootstrapStateRoot := baseTS.ToSlice()[0].StateRoot
	pSt, err := state.LoadStateTree(ctx, cst, baseTS.ToSlice()[0].StateRoot, builtin.Actors)
	require.NoError(t, err)
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
	require.NoError(t, err)

	wFun := func(ts types.TipSet) (uint64, error) {
		// No power-altering messages processed from here on out.
		// And so bootstrapSt correctly retrives power table for all
		// test blocks.
		return con.Weight(ctx, ts, pSt)
	}

	fakeChildParams := th.FakeChildParams{
		Parent:     baseTS,
		GenesisCid: calcGenBlk.Cid(),
		StateRoot:  bootstrapStateRoot,
		Signer:     mockSigner,

		MinerAddr:   info.Miners[1].Address,
		MinerWorker: minerWorker1,
	}

	f1b1 := th.RequireMkFakeChildCore(t, fakeChildParams, wFun)
	f1b1.Proof, f1b1.Ticket, err = th.MakeProofAndWinningTicket(minerWorker1, info.Miners[1].Power, totalPower, mockSigner)
	require.NoError(t, err)

	fakeChildParams.Nonce = uint64(1)
	fakeChildParams.MinerAddr = info.Miners[2].Address
	f2b1 := th.RequireMkFakeChildCore(t, fakeChildParams, wFun)
	f2b1.Proof, f2b1.Ticket, err = th.MakeProofAndWinningTicket(minerWorker1, info.Miners[2].Power, totalPower, mockSigner)
	require.NoError(t, err)

	tsShared := th.RequireNewTipSet(t, f1b1, f2b1)

	// Sync first tipset, should have weight 22 + starting
	sharedCids := requirePutBlocks(t, blockSource, f1b1, f2b1)
	err = syncer.HandleNewTipset(ctx, sharedCids)
	require.NoError(t, err)
	assertHead(t, chainStore, tsShared)
	measuredWeight, err := wFun(requireHeadTipset(t, chainStore))
	require.NoError(t, err)
	expectedWeight := startingWeight + uint64(22000)
	assert.Equal(t, expectedWeight, measuredWeight)

	// fork 1 is heavier than the old head.
	fakeChildParams = th.FakeChildParams{
		Parent:     th.RequireNewTipSet(t, f1b1),
		GenesisCid: calcGenBlk.Cid(),
		StateRoot:  bootstrapStateRoot,
		Signer:     mockSigner,

		MinerAddr:   info.Miners[1].Address,
		MinerWorker: minerWorker1,
	}
	f1b2a := th.RequireMkFakeChildCore(t, fakeChildParams, wFun)
	f1b2a.Proof, f1b2a.Ticket, err = th.MakeProofAndWinningTicket(minerWorker1, info.Miners[1].Power, totalPower, mockSigner)
	require.NoError(t, err)

	fakeChildParams.Nonce = uint64(1)

	fakeChildParams.MinerAddr = info.Miners[2].Address
	fakeChildParams.MinerWorker = minerWorker2
	f1b2b := th.RequireMkFakeChildCore(t, fakeChildParams, wFun)
	f1b2b.Proof, f1b2b.Ticket, err = th.MakeProofAndWinningTicket(minerWorker2, info.Miners[2].Power, totalPower, mockSigner)
	require.NoError(t, err)

	f1 := th.RequireNewTipSet(t, f1b2a, f1b2b)
	f1Cids := requirePutBlocks(t, blockSource, f1.ToSlice()...)
	err = syncer.HandleNewTipset(ctx, f1Cids)
	require.NoError(t, err)
	assertHead(t, chainStore, f1)
	measuredWeight, err = wFun(requireHeadTipset(t, chainStore))
	require.NoError(t, err)
	expectedWeight = startingWeight + uint64(33000)
	assert.Equal(t, expectedWeight, measuredWeight)

	// fork 2 has heavier weight because of addr3's power even though there
	// are fewer blocks in the tipset than fork 1.
	fakeChildParams = th.FakeChildParams{
		Parent:     th.RequireNewTipSet(t, f2b1),
		GenesisCid: calcGenBlk.Cid(),
		Signer:     mockSigner,

		StateRoot:   bootstrapStateRoot,
		MinerAddr:   info.Miners[3].Address,
		MinerWorker: minerWorker2,
	}
	f2b2 := th.RequireMkFakeChildCore(t, fakeChildParams, wFun)
	f2b2.Proof, f2b2.Ticket, err = th.MakeProofAndWinningTicket(minerWorker2, info.Miners[3].Power, totalPower, mockSigner)
	require.NoError(t, err)

	f2 := th.RequireNewTipSet(t, f2b2)
	f2Cids := requirePutBlocks(t, blockSource, f2.ToSlice()...)
	err = syncer.HandleNewTipset(ctx, f2Cids)
	require.NoError(t, err)
	assertHead(t, chainStore, f2)
	measuredWeight, err = wFun(requireHeadTipset(t, chainStore))
	require.NoError(t, err)
	expectedWeight = startingWeight + uint64(119000)
	assert.Equal(t, expectedWeight, measuredWeight)
}

type tipSetGetter interface {
	GetTipSet(types.TipSetKey) (types.TipSet, error)
}

func requireGetTipSet(ctx context.Context, t *testing.T, chainStore tipSetGetter, key types.TipSetKey) types.TipSet {
	ts, err := chainStore.GetTipSet(key)
	require.NoError(t, err)
	return ts
}

type tipSetStateRootGetter interface {
	GetTipSetStateRoot(tsKey types.TipSetKey) (cid.Cid, error)
}

func requireGetTipSetStateRoot(ctx context.Context, t *testing.T, chainStore tipSetStateRootGetter, key types.TipSetKey) cid.Cid {
	stateCid, err := chainStore.GetTipSetStateRoot(key)
	require.NoError(t, err)
	return stateCid
}

func initGenesis(minerAddress address.Address, minerOwnerAddress address.Address, minerPeerID peer.ID, cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error) {
	return consensus.MakeGenesisFunc(
		consensus.MinerActor(minerAddress, minerOwnerAddress, minerPeerID, types.ZeroAttoFIL, types.OneKiBSectorSize),
	)(cst, bs)
}
