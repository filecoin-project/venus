package gengen_test

import (
	"context"
	"io/ioutil"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmSz8kAe2JCKp2dWSG8gHSWnwSmne8YfRXTeK5HBmc9L7t/go-ipfs-exchange-offline"
	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	bserv "gx/ipfs/QmZsGVGCqMCNzHLNMB6q4F6yyvomqf1VxwhJwSfgo1NGaF/go-blockservice"

	. "github.com/filecoin-project/go-filecoin/gengen/util"
	th "github.com/filecoin-project/go-filecoin/testhelpers"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
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
