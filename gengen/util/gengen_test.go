package gengen_test

import (
	"context"
	"io/ioutil"
	"testing"

	bserv "github.com/ipfs/go-blockservice"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-offline"

	. "github.com/filecoin-project/go-filecoin/gengen/util"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"

	"github.com/stretchr/testify/assert"
)

var testConfig = &GenesisCfg{
	Keys:     4,
	PreAlloc: []string{"10", "50"},
	Miners: []Miner{
		{
			Owner: 0,
			Power: 50,
		},
		{
			Owner: 1,
			Power: 10,
		},
	},
}

func TestGenGenLoading(t *testing.T) {
	tf.IntegrationTest(t)

	assert := assert.New(t)
	fi, err := ioutil.TempFile("", "gengentest")
	assert.NoError(err)

	_, err = GenGenesisCar(testConfig, fi, 0)
	assert.NoError(err)
	assert.NoError(fi.Close())

	td := th.NewDaemon(t, th.GenesisFile(fi.Name())).Start()
	defer td.ShutdownSuccess()

	o := td.Run("actor", "ls").AssertSuccess()

	stdout := o.ReadStdout()
	assert.Contains(stdout, `"MinerActor"`)
	assert.Contains(stdout, `"StoragemarketActor"`)
}

func TestGenGenDeterministicBetweenBuilds(t *testing.T) {
	tf.UnitTest(t)

	assert := assert.New(t)

	var info *RenderedGenInfo
	for i := 0; i < 50; i++ {
		mds := ds.NewMapDatastore()
		bstore := blockstore.NewBlockstore(mds)
		offl := offline.Exchange(bstore)
		blkserv := bserv.New(bstore, offl)
		cst := &hamt.CborIpldStore{Blocks: blkserv}

		ctx := context.Background()

		inf, err := GenGen(ctx, testConfig, cst, bstore, 0)
		assert.NoError(err)
		if info == nil {
			info = inf
		} else {
			assert.Equal(info, inf)
		}
	}
}
