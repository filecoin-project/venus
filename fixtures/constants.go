package fixtures

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

// The file used to build these addresses can be found in:
// $GOPATH/src/github.com/filecoin-project/go-filecoin/fixtures/setup.json
//
// If said file is modified these addresses will need to change as well
// rebuild using
// TODO: move to build script
// https://github.com/filecoin-project/go-filecoin/issues/921
// cat ./fixtures/setup.json | ./gengen/gengen --json --keypath fixtures > fixtures/genesis.car 2> fixtures/gen.json

// TestAddresses is a list of pregenerated addresses.
var TestAddresses []string

// testKeys is a list of filenames, which contain the private keys of the pregenerated addresses.
var testKeys []string

// TestMiners is a list of pregenerated miner acccounts. They are owned by the matching TestAddress.
var TestMiners []string

type detailsStruct struct {
	Keys   []*types.KeyInfo
	Miners []struct {
		Owner   int
		Address string
		Power   uint64
	}
	GenesisCid cid.Cid `refmt:",omitempty"`
}

func init() {
	gopath, err := th.GetGoPath()
	if err != nil {
		panic(err)
	}

	detailspath := filepath.Join(gopath, "/src/github.com/filecoin-project/go-filecoin/fixtures/gen.json")
	detailsFile, err := os.Open(detailspath)
	if err != nil {
		// fmt.Printf("Fixture data not found. Skipping fixture initialization: %s\n", err)
		return
	}
	defer func() {
		if err := detailsFile.Close(); err != nil {
			panic(err)
		}
	}()
	detailsFileBytes, err := ioutil.ReadAll(detailsFile)
	if err != nil {
		panic(err)
	}
	var details detailsStruct
	if err := json.Unmarshal(detailsFileBytes, &details); err != nil {
		panic(err)
	}

	var keys []int
	for key := range details.Keys {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	miners := details.Miners

	for _, key := range keys {
		info := details.Keys[key]
		addr, err := info.Address()
		if err != nil {
			panic(err)
		}
		TestAddresses = append(TestAddresses, addr.String())
		testKeys = append(testKeys, fmt.Sprintf("%d.key", key))
	}

	for _, miner := range miners {
		TestMiners = append(TestMiners, miner.Address)
	}
}

// KeyFilePaths returns the paths to the wallets of the testaddresses
func KeyFilePaths() []string {
	gopath, err := th.GetGoPath()
	if err != nil {
		panic(err)
	}
	folder := "/src/github.com/filecoin-project/go-filecoin/fixtures/"

	res := make([]string, len(testKeys))
	for i, k := range testKeys {
		res[i] = filepath.Join(gopath, folder, k)
	}

	return res
}

// test devnet addrs
const (
	testFilecoinBootstrap0 string = "/dns4/test.kittyhawk.wtf/tcp/9000/ipfs/Qmd6xrWYHsxivfakYRy6MszTpuAiEoFbgE1LWw4EvwBpp4"
	testFilecoinBootstrap1 string = "/dns4/test.kittyhawk.wtf/tcp/9001/ipfs/QmXq6XEYeEmUzBFuuKbVEGgxEpVD4xbSkG2Rhek6zkFMp4"
	testFilecoinBootstrap2 string = "/dns4/test.kittyhawk.wtf/tcp/9002/ipfs/QmXhxqTKzBKHA5FcMuiKZv8YaMPwpbKGXHRVZcFB2DX9XY"
	testFilecoinBootstrap3 string = "/dns4/test.kittyhawk.wtf/tcp/9003/ipfs/QmZGDLdQLUTi7uYTNavKwCd7SBc5KMfxzWxAyvqRQvwuiV"
	testFilecoinBootstrap4 string = "/dns4/test.kittyhawk.wtf/tcp/9004/ipfs/QmZRnwmCjyNHgeNDiyT8mXRtGhP6uSzgHtrozc42crmVbg"
)

// nightly devnet addrs
const (
	nightlyFilecoinBootstrap0 string = "/dns4/nightly.kittyhawk.wtf/tcp/9000/ipfs/Qmd6xrWYHsxivfakYRy6MszTpuAiEoFbgE1LWw4EvwBpp4"
	nightlyFilecoinBootstrap1 string = "/dns4/nightly.kittyhawk.wtf/tcp/9001/ipfs/QmXq6XEYeEmUzBFuuKbVEGgxEpVD4xbSkG2Rhek6zkFMp4"
	nightlyFilecoinBootstrap2 string = "/dns4/nightly.kittyhawk.wtf/tcp/9002/ipfs/QmXhxqTKzBKHA5FcMuiKZv8YaMPwpbKGXHRVZcFB2DX9XY"
	nightlyFilecoinBootstrap3 string = "/dns4/nightly.kittyhawk.wtf/tcp/9003/ipfs/QmZGDLdQLUTi7uYTNavKwCd7SBc5KMfxzWxAyvqRQvwuiV"
	nightlyFilecoinBootstrap4 string = "/dns4/nightly.kittyhawk.wtf/tcp/9004/ipfs/QmZRnwmCjyNHgeNDiyT8mXRtGhP6uSzgHtrozc42crmVbg"
)

// user devnet addrs
const (
	userFilecoinBootstrap0 string = "/dns4/user.kittyhawk.wtf/tcp/9000/ipfs/Qmd6xrWYHsxivfakYRy6MszTpuAiEoFbgE1LWw4EvwBpp4"
	userFilecoinBootstrap1 string = "/dns4/user.kittyhawk.wtf/tcp/9001/ipfs/QmXq6XEYeEmUzBFuuKbVEGgxEpVD4xbSkG2Rhek6zkFMp4"
	userFilecoinBootstrap2 string = "/dns4/user.kittyhawk.wtf/tcp/9002/ipfs/QmXhxqTKzBKHA5FcMuiKZv8YaMPwpbKGXHRVZcFB2DX9XY"
	userFilecoinBootstrap3 string = "/dns4/user.kittyhawk.wtf/tcp/9003/ipfs/QmZGDLdQLUTi7uYTNavKwCd7SBc5KMfxzWxAyvqRQvwuiV"
	userFilecoinBootstrap4 string = "/dns4/user.kittyhawk.wtf/tcp/9004/ipfs/QmZRnwmCjyNHgeNDiyT8mXRtGhP6uSzgHtrozc42crmVbg"
)

// DevnetTestBootstrapAddrs are the dns multiaddrs for the nodes of the filecoin
// test devnet.
var DevnetTestBootstrapAddrs = []string{
	testFilecoinBootstrap0,
	testFilecoinBootstrap1,
	testFilecoinBootstrap2,
	testFilecoinBootstrap3,
	testFilecoinBootstrap4,
}

// DevnetNightlyBootstrapAddrs are the dns multiaddrs for the nodes of the filecoin
// nightly devnet
var DevnetNightlyBootstrapAddrs = []string{
	nightlyFilecoinBootstrap0,
	nightlyFilecoinBootstrap1,
	nightlyFilecoinBootstrap2,
	nightlyFilecoinBootstrap3,
	nightlyFilecoinBootstrap4,
}

// DevnetUserBootstrapAddrs are the dns multiaddrs for the nodes of the filecoin
// user devnet
var DevnetUserBootstrapAddrs = []string{
	userFilecoinBootstrap0,
	userFilecoinBootstrap1,
	userFilecoinBootstrap2,
	userFilecoinBootstrap3,
	userFilecoinBootstrap4,
}
