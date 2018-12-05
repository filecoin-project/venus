package gengen_test

import (
	"context"
	"io/ioutil"
	"testing"

	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	"gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	bserv "gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	"gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"
	ds "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"

	. "github.com/filecoin-project/go-filecoin/gengen/util"
	th "github.com/filecoin-project/go-filecoin/testhelpers"

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
	assert := assert.New(t)
	fi, err := ioutil.TempFile("", "gengentest")
	assert.NoError(err)

	_, err = GenGenesisCar(testConfig, fi, 0)
	assert.NoError(err)
	assert.NoError(fi.Close())

	td := th.NewDaemon(t, th.GenesisFile(fi.Name())).Start()
	defer td.Shutdown()

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
