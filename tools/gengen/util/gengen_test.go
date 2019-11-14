package gengen_test

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"

	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	. "github.com/filecoin-project/go-filecoin/tools/gengen/util"

	"github.com/stretchr/testify/assert"
)

var defaultGenesisTime = time.Unix(123456789, 0)

var testConfig = &GenesisCfg{
	ProofsMode: types.TestProofsMode,
	Keys:       4,
	PreAlloc:   []string{"10", "50"},
	Miners: []*CreateStorageMinerConfig{
		{
			Owner:               0,
			NumCommittedSectors: 50,
			SectorSize:          types.OneKiBSectorSize.Uint64(),
		},
		{
			Owner:               1,
			NumCommittedSectors: 10,
			SectorSize:          types.OneKiBSectorSize.Uint64(),
		},
	},
	Network: "go-filecoin-test",
}

func TestGenGenLoading(t *testing.T) {
	tf.IntegrationTest(t)

	fi, err := ioutil.TempFile("", "gengentest")
	assert.NoError(t, err)

	_, err = GenGenesisCar(testConfig, fi, 0, defaultGenesisTime)
	assert.NoError(t, err)
	assert.NoError(t, fi.Close())

	td := th.NewDaemon(t, th.GenesisFile(fi.Name())).Start()
	defer td.ShutdownSuccess()

	o := td.Run("actor", "ls").AssertSuccess()

	stdout := o.ReadStdout()
	assert.Contains(t, stdout, `"MinerActor"`)
	assert.Contains(t, stdout, `"StoragemarketActor"`)
	assert.Contains(t, stdout, `"InitActor"`)
}

func TestGenGenDeterministicBetweenBuilds(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	var info *RenderedGenInfo
	for i := 0; i < 50; i++ {
		bstore := blockstore.NewBlockstore(ds.NewMapDatastore())
		cst := hamt.CSTFromBstore(bstore)
		inf, err := GenGen(ctx, testConfig, cst, bstore, 0, defaultGenesisTime)
		assert.NoError(t, err)
		if info == nil {
			info = inf
		} else {
			assert.Equal(t, info, inf)
		}
	}
}
