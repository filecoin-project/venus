package chain

import (
	"context"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"testing"

	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTotal(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx := context.Background()

	power := uint64(19)
	bs, _, st := requireMinerWithPower(ctx, t, power)

	actual, err := (&consensus.MarketView{}).Total(ctx, st, bs)
	require.NoError(err)

	assert.Equal(power, actual)
}

func TestMiner(t *testing.T) {
	ctx := context.Background()
	require := require.New(t)
	assert := assert.New(t)

	power := uint64(12)
	bs, addr, st := requireMinerWithPower(ctx, t, power)

	actual, err := (&consensus.MarketView{}).Miner(ctx, st, bs, addr)
	require.NoError(err)

	assert.Equal(power, actual)
}

func requireMinerWithPower(ctx context.Context, t *testing.T, power uint64) (bstore.Blockstore, address.Address, state.Tree) {

	// set up genesis block with power
	require := require.New(t)

	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()

	// BEGIN Lifted from default_syncer_test.go
	// set up genesis block with power
	ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
	mockSigner := types.NewMockSigner(ki)
	testAddress := mockSigner.Addresses[0]

	testGen := consensus.MakeGenesisFunc(
		consensus.ActorAccount(testAddress, types.NewAttoFILFromFIL(10000)),
	)

	// chain.Store
	calcGenBlk, err := testGen(cst, bs) // flushes state
	require.NoError(err)
	chainDS := r.ChainDatastore()
	chain := NewDefaultStore(chainDS, cst, calcGenBlk.Cid())

	// chain.Syncer
	processor := testhelpers.NewTestProcessor()
	prover := proofs.NewFakeProver(true, nil)
	con := consensus.NewExpected(cst, bs, processor, &testhelpers.TestView{}, calcGenBlk.Cid(), prover)
	syncer := NewDefaultSyncer(cst, cst, con, chain) // note we use same cst for on and offline for tests

	// Initialize stores to contain genesis block and state
	calcGenTS := testhelpers.RequireNewTipSet(require, calcGenBlk)
	genTsas := &TipSetAndState{
		TipSet:          calcGenTS,
		TipSetStateRoot: calcGenBlk.StateRoot,
	}
	RequirePutTsas(ctx, require, chain, genTsas)
	err = chain.SetHead(ctx, calcGenTS) // Initialize chain store with correct genesis
	require.NoError(err)
	requireHead(require, chain, calcGenTS)
	requireTsAdded(require, chain, calcGenTS)

	calcGenBlkCid := calcGenBlk.Cid()
	// END Lifted from default_syncer_test.go

	addr0, block, nonce, err := CreateMinerWithPower(ctx, t, syncer, calcGenBlk, mockSigner, 0, mockSigner.Addresses[0], uint64(0), cst, bs, calcGenBlkCid)
	require.NoError(err)

	addrMine, _, _, err := CreateMinerWithPower(ctx, t, syncer, block, mockSigner, nonce, addr0, power, cst, bs, calcGenBlkCid)
	require.NoError(err)

	st, err := chain.LatestState(ctx)
	require.NoError(err)
	return bs, addrMine, st
}
