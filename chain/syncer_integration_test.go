package chain_test

import (
	"context"
	"testing"
	"time"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-offline"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"

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
	cborStore := hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	store := chain.NewStore(repo.ChainDatastore(), &cborStore, &state.TreeStateLoader{}, chain.NewStatusReporter(), genesis.At(0).Cid())
	require.NoError(t, store.PutTipSetAndState(ctx, &chain.TipSetAndState{genStateRoot, genesis}))
	require.NoError(t, store.SetHead(ctx, genesis))

	// Note: the chain builder is passed as the fetcher, from which blocks may be requested, but
	// *not* as the store, to which the syncer must ensure to put blocks.
	eval := &chain.FakeStateEvaluator{}
	syncer := chain.NewSyncer(eval, store, builder, builder, chain.NewStatusReporter(), th.NewFakeClock(time.Unix(1234567890, 0)))

	base := builder.AppendManyOn(3, genesis)
	left := builder.AppendManyOn(4, base)
	right := builder.AppendManyOn(3, base)

	// Sync the two branches, which stores all blocks in the underlying stores.
	assert.NoError(t, syncer.HandleNewTipSet(ctx, types.NewChainInfo("", left.Key(), heightFromTip(t, left)), true))
	assert.NoError(t, syncer.HandleNewTipSet(ctx, types.NewChainInfo("", right.Key(), heightFromTip(t, right)), true))
	verifyHead(t, store, left)

	// The syncer/store assume that the fetcher populates the underlying block store such that
	// tipsets can be reconstructed. The chain builder used for testing doesn't do that, so do
	// it manually here.
	for _, tip := range []types.TipSet{left, right} {
		for itr := chain.IterAncestors(ctx, builder, tip); !itr.Complete(); require.NoError(t, itr.Next()) {
			for _, block := range itr.Value().ToSlice() {
				_, err := cborStore.Put(ctx, block)
				require.NoError(t, err)
			}
		}
	}

	// Load a new chain store on the underlying data. It will only compute state for the
	// left (heavy) branch. It has a fetcher that can't provide blocks.
	newStore := chain.NewStore(repo.ChainDatastore(), &cborStore, &state.TreeStateLoader{}, chain.NewStatusReporter(), genesis.At(0).Cid())
	require.NoError(t, newStore.Load(ctx))
	fakeFetcher := th.NewTestFetcher()
	offlineSyncer := chain.NewSyncer(eval, newStore, builder, fakeFetcher, chain.NewStatusReporter(), th.NewFakeClock(time.Unix(1234567890, 0)))

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

	err = offlineSyncer.HandleNewTipSet(ctx, types.NewChainInfo("", newRight.Key(), heightFromTip(t, newRight)), true)
	assert.Error(t, err)

	// The left chain is ok without any fetching though.
	assert.NoError(t, offlineSyncer.HandleNewTipSet(ctx, types.NewChainInfo("", left.Key(), heightFromTip(t, left)), true))
}

// Syncer handles MarketView weight comparisons.
// Current issue: when creating miner mining with addr0, addr0's storage head isn't found in the blockstore
// and I can't figure out why because we pass in the correct blockstore to createStorageMinerWithpower.
func TestTipSetWeightDeep(t *testing.T) {
	// This test takes many seconds, the bottleneck is gengen.
	tf.IntegrationTest(t)

	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}

	ctx := context.Background()

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
		Network: "syncerIntegrationTest",
	}

	info, err := gengen.GenGen(ctx, genCfg, cst, bs, 0)
	require.NoError(t, err)

	minerWorker1, err := info.Keys[0].Address()
	require.NoError(t, err)
	minerWorker2 := minerWorker1
	// Include gengen miner worker key so that we can correctly sign blocks
	mockSigner := types.NewMockSigner([]types.KeyInfo{*info.Keys[0]})

	var calcGenBlk types.Block
	require.NoError(t, cst.Get(ctx, info.GenesisCid, &calcGenBlk))

	chainStore := chain.NewStore(r.ChainDatastore(), cst, &state.TreeStateLoader{}, chain.NewStatusReporter(), calcGenBlk.Cid())
	messageStore := chain.NewMessageStore(cst)
	emptyMessagesCid, err := messageStore.StoreMessages(ctx, []*types.SignedMessage{})
	require.NoError(t, err)
	emptyReceiptsCid, err := messageStore.StoreReceipts(ctx, []*types.MessageReceipt{})
	require.NoError(t, err)

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
	con := consensus.NewExpected(cst, bs, th.NewTestProcessor(), th.NewFakeBlockValidator(), &consensus.MarketView{}, calcGenBlk.Cid(), th.BlockTimeTest, &consensus.FakeElectionMachine{}, &consensus.FakeTicketMachine{})
	syncer := chain.NewSyncer(con, chainStore, messageStore, blockSource, chain.NewStatusReporter(), th.NewFakeClock(time.Unix(1234567890, 0)))
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
	var f1b1Ticket types.Ticket
	f1b1.ElectionProof, f1b1Ticket = consensus.MakeFakeElectionProofForTest(), consensus.MakeFakeTicketForTest()
	f1b1.Tickets = []types.Ticket{f1b1Ticket}
	f1b1.Messages = emptyMessagesCid
	f1b1.MessageReceipts = emptyReceiptsCid
	f1b1.BlockSig, err = mockSigner.SignBytes(f1b1.SignatureData(), minerWorker1)
	require.NoError(t, err)

	fakeChildParams.MinerAddr = info.Miners[2].Address
	f2b1 := th.RequireMkFakeChildCore(t, fakeChildParams, wFun)
	var f2b1Ticket types.Ticket
	f2b1.ElectionProof, f2b1Ticket = consensus.MakeFakeElectionProofForTest(), consensus.MakeFakeTicketForTest()
	f2b1.Tickets = []types.Ticket{f2b1Ticket}
	f2b1.Messages = emptyMessagesCid
	f2b1.MessageReceipts = emptyReceiptsCid
	f2b1.BlockSig, err = mockSigner.SignBytes(f2b1.SignatureData(), minerWorker1)
	require.NoError(t, err)

	tsShared := th.RequireNewTipSet(t, f1b1, f2b1)

	// Sync first tipset, should have weight 22 + starting
	sharedCids := requirePutBlocks(t, blockSource, f1b1, f2b1)
	err = syncer.HandleNewTipSet(ctx, types.NewChainInfo(peer.ID(""), sharedCids, uint64(f1b1.Height)), true)
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
	var f1b2aTicket types.Ticket
	f1b2a.ElectionProof, f1b2aTicket = consensus.MakeFakeElectionProofForTest(), consensus.MakeFakeTicketForTest()
	f1b2a.Tickets = []types.Ticket{f1b2aTicket}
	f1b2a.Messages = emptyMessagesCid
	f1b2a.MessageReceipts = emptyReceiptsCid
	f1b2a.BlockSig, err = mockSigner.SignBytes(f1b2a.SignatureData(), minerWorker1)
	require.NoError(t, err)

	fakeChildParams.MinerAddr = info.Miners[2].Address
	fakeChildParams.MinerWorker = minerWorker2
	f1b2b := th.RequireMkFakeChildCore(t, fakeChildParams, wFun)
	var f1b2bTicket types.Ticket
	f1b2b.ElectionProof, f1b2bTicket = consensus.MakeFakeElectionProofForTest(), consensus.MakeFakeTicketForTest()
	require.NoError(t, err)
	f1b2b.Tickets = []types.Ticket{f1b2bTicket}
	f1b2b.Messages = emptyMessagesCid
	f1b2b.MessageReceipts = emptyReceiptsCid
	f1b2b.BlockSig, err = mockSigner.SignBytes(f1b2b.SignatureData(), minerWorker1)
	require.NoError(t, err)

	f1 := th.RequireNewTipSet(t, f1b2a, f1b2b)
	f1Cids := requirePutBlocks(t, blockSource, f1.ToSlice()...)

	f1H, err := f1.Height()
	require.NoError(t, err)
	err = syncer.HandleNewTipSet(ctx, types.NewChainInfo(peer.ID(""), f1Cids, f1H), true)
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
	var f2b2Ticket types.Ticket
	f2b2.ElectionProof, f2b2Ticket = consensus.MakeFakeElectionProofForTest(), consensus.MakeFakeTicketForTest()
	f2b2.Tickets = []types.Ticket{f2b2Ticket}
	f2b2.Messages = emptyMessagesCid
	f2b2.MessageReceipts = emptyReceiptsCid
	f2b2.BlockSig, err = mockSigner.SignBytes(f2b2.SignatureData(), minerWorker2)
	require.NoError(t, err)

	f2 := th.RequireNewTipSet(t, f2b2)
	f2Cids := requirePutBlocks(t, blockSource, f2.ToSlice()...)

	f2H, err := f2.Height()
	require.NoError(t, err)
	err = syncer.HandleNewTipSet(ctx, types.NewChainInfo(peer.ID(""), f2Cids, f2H), true)
	require.NoError(t, err)

	assertHead(t, chainStore, f2)
	measuredWeight, err = wFun(requireHeadTipset(t, chainStore))
	require.NoError(t, err)
	expectedWeight = startingWeight + uint64(119000)
	assert.Equal(t, expectedWeight, measuredWeight)
}

type requireTsAddedChainStore interface {
	GetTipSet(types.TipSetKey) (types.TipSet, error)
	GetTipSetAndStatesByParentsAndHeight(types.TipSetKey, uint64) ([]*chain.TipSetAndState, error)
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
	childTsasSlice, err := chain.GetTipSetAndStatesByParentsAndHeight(parent, h)

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
