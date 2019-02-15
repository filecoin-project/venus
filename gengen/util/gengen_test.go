package gengen_test

import (
	"context"
	"io/ioutil"
	"testing"

	"gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	"gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"
	bserv "gx/ipfs/QmZuPasxd7fSgtzRzCL7Z8J8QwDJML2fgBUExRbQCqb4BT/go-blockservice"
	"gx/ipfs/QmeoCaPwsaPtW34W4vnPEYFYNgNFAygknmX2RRBbGytF9Y/go-hamt-ipld"
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
