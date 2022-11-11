package state_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus/pkg/testhelpers"

	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/state"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	gengen "github.com/filecoin-project/venus/tools/gengen/util"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func TestTotal(t *testing.T) {
	// todo think a way to mock power directly
	t.Skipf("skip it due to cant mock power directly ")
	tf.UnitTest(t)

	ctx := context.Background()
	numCommittedSectors := uint64(19)
	numMiners := 3
	kis := testhelpers.MustGenerateBLSKeyInfo(numMiners, 0)

	cst, _, root := requireMinerWithNumCommittedSectors(ctx, t, numCommittedSectors, kis)

	table := state.NewPowerTableView(state.NewView(cst, root), state.NewView(cst, root))
	networkPower, err := table.NetworkTotalPower(ctx)
	require.NoError(t, err)

	// TODO: test that the QA power is used when it differs from raw byte power after gengen computes it properly
	// https://github.com/filecoin-project/venus/issues/4011
	expected := big.NewIntUnsigned(uint64(constants.DevSectorSize) * numCommittedSectors * uint64(numMiners))
	assert.True(t, expected.Equals(networkPower))
}

func TestMiner(t *testing.T) {
	// todo think a way to mock power directly
	t.Skipf("skip it due to cant mock power directly ")
	tf.UnitTest(t)

	ctx := context.Background()
	kis := testhelpers.MustGenerateBLSKeyInfo(1, 0)

	numCommittedSectors := uint64(10)
	cst, addrs, root := requireMinerWithNumCommittedSectors(ctx, t, numCommittedSectors, kis)
	addr := addrs[0]

	table := state.NewPowerTableView(state.NewView(cst, root), state.NewView(cst, root))
	actual, err := table.MinerClaimedPower(ctx, addr)
	require.NoError(t, err)

	expected := abi.NewStoragePower(int64(uint64(constants.DevSectorSize) * numCommittedSectors))
	assert.True(t, expected.Equals(actual))
	assert.Equal(t, expected, actual)
}

func TestNoPowerAfterSlash(t *testing.T) {
	// todo think a way to mock power directly
	t.Skipf("skip it due to cant mock power directly ")
	tf.UnitTest(t)
	// setup lookback state with 3 miners
	ctx := context.Background()
	numCommittedSectors := uint64(19)
	numMiners := 3
	kis := testhelpers.MustGenerateBLSKeyInfo(numMiners, 0)
	cstPower, addrsPower, rootPower := requireMinerWithNumCommittedSectors(ctx, t, numCommittedSectors, kis)
	cstFaults, _, rootFaults := requireMinerWithNumCommittedSectors(ctx, t, numCommittedSectors, kis[0:2]) // drop the third key
	table := state.NewPowerTableView(state.NewView(cstPower, rootPower), state.NewView(cstFaults, rootFaults))

	// verify that faulted miner claim is 0 power
	claim, err := table.MinerClaimedPower(ctx, addrsPower[2])
	require.NoError(t, err)
	assert.Equal(t, abi.NewStoragePower(0), claim)
}

func TestTotalPowerUnaffectedBySlash(t *testing.T) {
	// todo think a way to mock power directly
	t.Skipf("skip it due to cant mock power directly ")
	tf.UnitTest(t)
	ctx := context.Background()
	numCommittedSectors := uint64(19)
	numMiners := 3
	kis := testhelpers.MustGenerateBLSKeyInfo(numMiners, 0)
	cstPower, _, rootPower := requireMinerWithNumCommittedSectors(ctx, t, numCommittedSectors, kis)
	cstFaults, _, rootFaults := requireMinerWithNumCommittedSectors(ctx, t, numCommittedSectors, kis[0:2]) // drop the third key
	table := state.NewPowerTableView(state.NewView(cstPower, rootPower), state.NewView(cstFaults, rootFaults))

	// verify that faulted miner claim is 0 power
	total, err := table.NetworkTotalPower(ctx)
	require.NoError(t, err)
	expected := abi.NewStoragePower(int64(uint64(constants.DevSectorSize) * numCommittedSectors * uint64(numMiners)))

	assert.Equal(t, expected, total)
}

// nolint
func requireMinerWithNumCommittedSectors(ctx context.Context, t *testing.T, numCommittedSectors uint64, ownerKeys []crypto.KeyInfo) (cbor.IpldStore, []address.Address, cid.Cid) {
	// todo think a way to mock power directly
	r := repo.NewInMemoryRepo()
	bs := r.Datastore()
	cst := cbor.NewCborStore(bs)

	numMiners := len(ownerKeys)
	minerConfigs := make([]*gengen.CreateStorageMinerConfig, numMiners)
	for i := 0; i < numMiners; i++ {
		commCfgs, err := gengen.MakeCommitCfgs(int(numCommittedSectors))
		require.NoError(t, err)
		minerConfigs[i] = &gengen.CreateStorageMinerConfig{
			Owner:            i,
			CommittedSectors: commCfgs,
			SealProofType:    constants.DevSealProofType,
			MarketBalance:    abi.NewTokenAmount(0),
		}
	}

	// set up genesis block containing some miners with non-zero power
	genCfg := &gengen.GenesisCfg{}
	require.NoError(t, gengen.MinerConfigs(minerConfigs)(genCfg))
	require.NoError(t, gengen.NetworkName("ptvtest")(genCfg))
	require.NoError(t, gengen.ImportKeys(ownerKeys, "1000000")(genCfg))

	info, err := gengen.GenGen(ctx, genCfg, bs)
	require.NoError(t, err)

	var genesis types.BlockHeader
	require.NoError(t, cst.Get(ctx, info.GenesisCid, &genesis))
	retAddrs := make([]address.Address, numMiners)
	for i := 0; i < numMiners; i++ {
		retAddrs[i] = info.Miners[i].Address
	}
	return cst, retAddrs, genesis.ParentStateRoot
}
