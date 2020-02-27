package consensus_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	gengen "github.com/filecoin-project/go-filecoin/tools/gengen/util"
)

func TestTotal(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("requires new init actor address resolution")

	ctx := context.Background()

	numCommittedSectors := uint64(19)
	cst, _, root := requireMinerWithNumCommittedSectors(ctx, t, numCommittedSectors)

	table := consensus.NewPowerTableView(state.NewView(cst, root))
	actual, err := table.Total(ctx)
	require.NoError(t, err)

	expected := abi.NewStoragePower(int64(uint64(constants.DevSectorSize) * numCommittedSectors))
	assert.True(t, expected.Equals(actual))
}

func TestMiner(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("requires new init actor address resolution")

	ctx := context.Background()

	numCommittedSectors := uint64(12)
	cst, addr, root := requireMinerWithNumCommittedSectors(ctx, t, numCommittedSectors)

	table := consensus.NewPowerTableView(state.NewView(cst, root))
	actual, err := table.MinerClaim(ctx, addr)
	require.NoError(t, err)

	expected := abi.NewStoragePower(int64(uint64(constants.DevSectorSize) * numCommittedSectors))
	assert.Equal(t, expected, actual)
}

func requireMinerWithNumCommittedSectors(ctx context.Context, t *testing.T, numCommittedSectors uint64) (*cborutil.IpldStore, address.Address, cid.Cid) {
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := cborutil.NewIpldStore(bs)
	commCfgs, err := gengen.MakeCommitCfgs(int(numCommittedSectors))
	require.NoError(t, err)

	// set up genesis block containing some miners with non-zero power
	genCfg := &gengen.GenesisCfg{
		ProofsMode: types.TestProofsMode,
		Keys:       1,
		Miners: []*gengen.CreateStorageMinerConfig{
			{
				CommittedSectors: commCfgs,
				SectorSize:       constants.DevSectorSize,
			},
		},
		Network: "ptvtest",
	}

	info, err := gengen.GenGen(ctx, genCfg, bs)
	require.NoError(t, err)

	var genesis block.Block
	require.NoError(t, cst.Get(ctx, info.GenesisCid, &genesis))
	return cst, info.Miners[0].Address, genesis.StateRoot.Cid
}
