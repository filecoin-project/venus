package gengen_test

import (
	"context"
	hamt "gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	ds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	offline "gx/ipfs/QmZxjqR9Qgompju73kakSoUj3rbVndAzky3oCDiBNCxPs1/go-ipfs-exchange-offline"
	blockstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
	"io/ioutil"
	"testing"

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

	_, err = GenGenesisCar(testConfig, fi)
	assert.NoError(err)
	assert.NoError(fi.Close())

	td := th.NewDaemon(t, th.GenesisFile(fi.Name())).Start()
	defer td.Shutdown()

	o := td.Run("actor", "ls").AssertSuccess()

	stdout := o.ReadStdout()
	assert.Contains(stdout, `"MinerActor"`)
	assert.Contains(stdout, `"StoragemarketActor"`)
}

func TestGenGenDeterministic(t *testing.T) {
	assert := assert.New(t)

	var info *RenderedGenInfo
	for i := 0; i < 50; i++ {
		mds := ds.NewMapDatastore()
		bstore := blockstore.NewBlockstore(mds)
		offl := offline.Exchange(bstore)
		blkserv := bserv.New(bstore, offl)
		cst := &hamt.CborIpldStore{Blocks: blkserv}

		ctx := context.Background()

		inf, err := GenGen(ctx, testConfig, cst, bstore)
		assert.NoError(err)
		if info == nil {
			info = inf
		} else {
			assert.Equal(info, inf)
		}
	}
}
