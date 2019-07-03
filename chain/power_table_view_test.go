package chain_test

import (
	"context"
	"testing"

	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTotal(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	numCommittedSectors := uint64(19)
	bs, _, st := requireMinerWithNumCommittedSectors(ctx, t, numCommittedSectors)

	actual, err := (&consensus.MarketView{}).Total(ctx, st, bs)
	require.NoError(t, err)

	expected := types.NewBytesAmount(types.OneKiBSectorSize.Uint64() * numCommittedSectors)

	assert.True(t, expected.Equal(actual))
}

func TestMiner(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	numCommittedSectors := uint64(12)
	bs, addr, st := requireMinerWithNumCommittedSectors(ctx, t, numCommittedSectors)

	actual, err := (&consensus.MarketView{}).Miner(ctx, st, bs, addr)
	require.NoError(t, err)

	expected := types.NewBytesAmount(types.OneKiBSectorSize.Uint64() * numCommittedSectors)

	assert.Equal(t, expected, actual)
}

func requireMinerWithNumCommittedSectors(ctx context.Context, t *testing.T, numCommittedSectors uint64) (bstore.Blockstore, address.Address, state.Tree) {
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()

	// set up genesis block containing some miners with non-zero power
	genCfg := &gengen.GenesisCfg{
		ProofsMode: types.TestProofsMode,
		Keys:       1,
		Miners: []*gengen.CreateStorageMinerConfig{
			{
				NumCommittedSectors: numCommittedSectors,
				SectorSize:          types.OneKiBSectorSize.Uint64(),
			},
		},
	}

	info, err := gengen.GenGen(ctx, genCfg, cst, bs, 0)
	require.NoError(t, err)

	var calcGenBlk types.Block
	require.NoError(t, cst.Get(ctx, info.GenesisCid, &calcGenBlk))

	stateTree, err := state.LoadStateTree(ctx, cst, calcGenBlk.StateRoot, builtin.Actors)
	require.NoError(t, err)

	return bs, info.Miners[0].Address, stateTree
}
