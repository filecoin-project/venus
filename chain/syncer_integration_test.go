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

// This file contains test that use a full syncer dependency structure, including store and
// consensus implementations.
// See syncer_test.go for most syncer unit tests.
// The tests here should probably be reworked to follow the unit test patterns, and this
// integration test setup reserved for a few "sunny day" tests of integration specifically.

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
	verifier := &verification.FakeVerifier{
		VerifyPoStValid: true,
	}
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
	verifier := &verification.FakeVerifier{
		VerifyPoStValid: true,
	}
	con := consensus.NewExpected(cst, bs, processor, th.NewFakeBlockValidator(), powerTable, dstP.genCid, verifier, th.BlockTimeTest)
	requireSetTestChain(t, con, false, dstP)
	initGenesisWrapper := func(cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error) {
		return initGenesis(dstP.minerAddress, dstP.minerOwnerAddress, dstP.minerPeerID, cst, bs)
	}
	return initSyncTest(t, con, initGenesisWrapper, cst, bs, r, dstP, chain.Syncing)
}

func initSyncTest(t *testing.T, con consensus.Protocol, genFunc func(cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error), cst *hamt.CborIpldStore, bs bstore.Blockstore, r repo.Repo, dstP *SyncerTestParams, syncMode chain.SyncMode) (*chain.Syncer, *chain.Store, repo.Repo, *th.TestFetcher) {
	ctx := context.Background()

	calcGenBlk, err := genFunc(cst, bs) // flushes state
	require.NoError(t, err)
	calcGenBlk.StateRoot = dstP.genStateRoot
	chainDS := r.ChainDatastore()
	chainStore := chain.NewStore(chainDS, cst, &state.TreeStateLoader{}, calcGenBlk.Cid())

	fetcher := th.NewTestFetcher()
	fetcher.AddSourceBlocks(calcGenBlk)
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

// Syncer is capable of recovering from a fork reorg after Load.
// See https://github.com/filecoin-project/go-filecoin/issues/1148#issuecomment-432008060
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
	// The fetcher will need to fetch these so that is may pass them to the
	// done callback inorder to determine when to stop fetching.
	// DONOTMERGE get specific approval on this before merging
	blockSource.AddSourceBlocks(forklink3.ToSlice()...)

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

	verifier := &verification.FakeVerifier{
		VerifyPoStValid: true,
	}
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
	verifier = &verification.FakeVerifier{
		VerifyPoStValid: true,
	}
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

func initGenesis(minerAddress address.Address, minerOwnerAddress address.Address, minerPeerID peer.ID, cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error) {
	return consensus.MakeGenesisFunc(
		consensus.MinerActor(minerAddress, minerOwnerAddress, minerPeerID, types.ZeroAttoFIL, types.OneKiBSectorSize),
	)(cst, bs)
}
