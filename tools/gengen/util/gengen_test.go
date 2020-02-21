package gengen_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/filecoin-project/specs-actors/actors/builtin"
	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	. "github.com/filecoin-project/go-filecoin/tools/gengen/util"

	"github.com/stretchr/testify/assert"
)

var testConfig = &GenesisCfg{
	ProofsMode: types.TestProofsMode,
	Keys:       4,
	PreAlloc:   []string{"10", "50"},
	Miners: []*CreateStorageMinerConfig{
		{
			Owner:               0,
			NumCommittedSectors: 50,
			SectorSize:          constants.DevSectorSize,
		},
		{
			Owner:               1,
			NumCommittedSectors: 10,
			SectorSize:          constants.DevSectorSize,
		},
	},
	Network: "go-filecoin-test",
	Seed:    defaultSeed,
	Time:    defaultTime,
}

const defaultSeed = 4
const defaultTime = 123456789

func TestGenGenLoading(t *testing.T) {
	tf.IntegrationTest(t)

	fi, err := ioutil.TempFile("", "gengentest")
	assert.NoError(t, err)

	_, err = GenGenesisCar(testConfig, fi)
	assert.NoError(t, err)
	assert.NoError(t, fi.Close())

	td := th.NewDaemon(t, th.GenesisFile(fi.Name())).Start()
	defer td.ShutdownSuccess()

	o := td.Run("actor", "ls").AssertSuccess()

	stdout := o.ReadStdout()
	assert.Contains(t, stdout, builtin.StoragePowerActorCodeID.String())
	assert.Contains(t, stdout, builtin.StorageMarketActorCodeID.String())
	assert.Contains(t, stdout, builtin.InitActorCodeID.String())
}

func TestGenGenDeterministicBetweenBuilds(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("Non-deterministic BLS key generation https://github.com/filecoin-project/go-filecoin/issues/3781")

	ctx := context.Background()
	var info *RenderedGenInfo
	for i := 0; i < 50; i++ {
		bstore := blockstore.NewBlockstore(ds.NewMapDatastore())
		cst := cborutil.NewIpldStore(bstore)
		inf, err := GenGen(ctx, testConfig, cst, bstore)
		assert.NoError(t, err)
		if info == nil {
			info = inf
		} else {
			assert.Equal(t, info, inf)
		}
	}
}
