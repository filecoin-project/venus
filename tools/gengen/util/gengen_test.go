package gengen_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/venus/pkg/constants"
	th "github.com/filecoin-project/venus/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	genutil "github.com/filecoin-project/venus/tools/gengen/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig(t *testing.T) *genutil.GenesisCfg {
	fiftyCommCfgs, err := genutil.MakeCommitCfgs(50)
	require.NoError(t, err)
	tenCommCfgs, err := genutil.MakeCommitCfgs(10)
	require.NoError(t, err)

	return &genutil.GenesisCfg{
		KeysToGen:         4,
		PreallocatedFunds: []string{"1000000", "500000"},
		Miners: []*genutil.CreateStorageMinerConfig{
			{
				Owner:            0,
				CommittedSectors: fiftyCommCfgs,
				SealProofType:    constants.DevSealProofType,
				MarketBalance:    abi.NewTokenAmount(0),
			},
			{
				Owner:            1,
				CommittedSectors: tenCommCfgs,
				SealProofType:    constants.DevSealProofType,
				MarketBalance:    abi.NewTokenAmount(0),
			},
		},
		Network: "gfctest",
		Seed:    defaultSeed,
		Time:    defaultTime,
	}
}

const defaultSeed = 4
const defaultTime = 123456789

func TestGenGenLoading(t *testing.T) {
	tf.IntegrationTest(t)

	fi, err := ioutil.TempFile("", "gengentest")
	assert.NoError(t, err)

	_, err = genutil.GenGenesisCar(testConfig(t), fi)
	assert.NoError(t, err)
	assert.NoError(t, fi.Close())

	td := th.NewDaemon(t, th.GenesisFile(fi.Name())).Start()
	defer td.ShutdownSuccess()

	o := td.Run("state", "list-actor").AssertSuccess()

	stdout := o.ReadStdout()
	assert.Contains(t, stdout, builtin2.StoragePowerActorCodeID.String())
	assert.Contains(t, stdout, builtin2.StorageMarketActorCodeID.String())
	assert.Contains(t, stdout, builtin2.InitActorCodeID.String())
}

func TestGenGenDeterministic(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	var info *genutil.RenderedGenInfo
	for i := 0; i < 5; i++ {
		bstore := blockstore.NewBlockstore(ds.NewMapDatastore())
		inf, err := genutil.GenGen(ctx, testConfig(t), bstore)
		assert.NoError(t, err)
		if info == nil {
			info = inf
		} else {
			assert.Equal(t, info.GenesisCid, inf.GenesisCid)
			assert.Equal(t, info.Miners, inf.Miners)
			assert.Equal(t, len(info.Keys), len(inf.Keys))
			for i, key := range inf.Keys {
				assert.Equal(t, info.Keys[i].Key(), key.Key())
			}
		}
	}
}
