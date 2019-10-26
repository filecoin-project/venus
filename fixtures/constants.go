package fixtures

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	cid "github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/build/project"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// The file used to build these addresses can be found in:
// $GOPATH/src/github.com/filecoin-project/go-filecoin/fixtures/setup.json
//
// If said file is modified these addresses will need to change as well
// rebuild using
// TODO: move to build script
// https://github.com/filecoin-project/go-filecoin/issues/921
// cat ./fixtures/setup.json | ./tools/gengen/gengen --json --keypath fixtures > fixtures/genesis.car 2> fixtures/gen.json

// TestAddresses is a list of pregenerated addresses.
var TestAddresses []string

// testKeys is a list of filenames, which contain the private keys of the pregenerated addresses.
var testKeys []string

// TestMiners is a list of pregenerated miner acccounts. They are owned by the matching TestAddress.
var TestMiners []string

type detailsStruct struct {
	Keys   []*types.KeyInfo
	Miners []struct {
		Owner               int
		Address             string
		NumCommittedSectors uint64
	}
	GenesisCid cid.Cid `refmt:",omitempty"`
}

func init() {
	root := project.Root()

	detailspath := filepath.Join(root, "fixtures/test/gen.json")
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
	root := project.Root()
	folder := filepath.Join(root, "fixtures/test")

	res := make([]string, len(testKeys))
	for i, k := range testKeys {
		res[i] = filepath.Join(folder, k)
	}

	return res
}

// staging devnet addrs
const (
	stagingFilecoinBootstrap0 string = "/dns4/staging.kittyhawk.wtf/tcp/9000/ipfs/Qmd6xrWYHsxivfakYRy6MszTpuAiEoFbgE1LWw4EvwBpp4"
	stagingFilecoinBootstrap1 string = "/dns4/staging.kittyhawk.wtf/tcp/9001/ipfs/QmXq6XEYeEmUzBFuuKbVEGgxEpVD4xbSkG2Rhek6zkFMp4"
	stagingFilecoinBootstrap2 string = "/dns4/staging.kittyhawk.wtf/tcp/9002/ipfs/QmXhxqTKzBKHA5FcMuiKZv8YaMPwpbKGXHRVZcFB2DX9XY"
	stagingFilecoinBootstrap3 string = "/dns4/staging.kittyhawk.wtf/tcp/9003/ipfs/QmZGDLdQLUTi7uYTNavKwCd7SBc5KMfxzWxAyvqRQvwuiV"
	stagingFilecoinBootstrap4 string = "/dns4/staging.kittyhawk.wtf/tcp/9004/ipfs/QmZRnwmCjyNHgeNDiyT8mXRtGhP6uSzgHtrozc42crmVbg"
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

// DevnetStagingBootstrapAddrs are the dns multiaddrs for the nodes of the filecoin
// staging devnet.
var DevnetStagingBootstrapAddrs = []string{
	stagingFilecoinBootstrap0,
	stagingFilecoinBootstrap1,
	stagingFilecoinBootstrap2,
	stagingFilecoinBootstrap3,
	stagingFilecoinBootstrap4,
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
