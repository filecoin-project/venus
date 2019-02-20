package chain_test

import (
	"context"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	bstore "gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
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
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	require := require.New(t)

	// set up genesis block with power
	genCfg := &gengen.GenesisCfg{
		Keys: 1,
		Miners: []gengen.Miner{
			{
				Power: power,
			},
		},
	}

	info, err := gengen.GenGen(ctx, genCfg, cst, bs, 0)
	require.NoError(err)

	var calcGenBlk types.Block
	require.NoError(cst.Get(ctx, info.GenesisCid, &calcGenBlk))

	stateTree, err := state.LoadStateTree(ctx, cst, calcGenBlk.StateRoot, builtin.Actors)
	require.NoError(err)

	return bs, info.Miners[0].Address, stateTree
}
