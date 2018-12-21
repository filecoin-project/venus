package chain

import (
	"context"
	"github.com/filecoin-project/go-filecoin/proofs"
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
	bootstrapPowerTable := consensus.NewTestPowerTableView(1, 1)
	require := require.New(t)

	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
	mockSigner := types.NewMockSigner(ki)
	testAddress := mockSigner.Addresses[0]

	testGen := consensus.MakeGenesisFunc(
		consensus.ActorAccount(testAddress, types.NewAttoFILFromFIL(10000)),
	)

	genBlk, err := testGen(cst, bs)
	require.NoError(err)
	genCid := genBlk.Cid()
	genesisTS := consensus.RequireNewTipSet(require, genBlk)
	genRoot := genBlk.StateRoot

	prover := proofs.NewFakeProver(true, nil)
	con := consensus.NewExpected(cst, bs, bootstrapPowerTable, genCid, prover)
	syncer, chain, cst, _ := initSyncTest(require, con, testGen, cst, bs, r)

	genTsas := &TipSetAndState{
		TipSet:          genesisTS,
		TipSetStateRoot: genRoot,
	}
	RequirePutTsas(ctx, require, chain, genTsas)
	err = chain.SetHead(ctx, genesisTS) // Initialize chain store with correct genesis
	require.NoError(err)
	requireHead(require, chain, genesisTS)
	requireTsAdded(require, chain, genesisTS)
	addrMine, _, _, err := CreateMinerWithPower(ctx, t, syncer, genBlk, mockSigner, uint64(0), mockSigner.Addresses[0], power, cst, bs, genCid)
	require.NoError(err)

	st, err := chain.LatestState(ctx)
	require.NoError(err)
	return bs, addrMine, st
}
