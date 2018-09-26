package fixtures

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

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
	Keys   map[string]*types.KeyInfo
	Miners []struct {
		Owner   string
		Address string
		Power   uint64
	}
	GenesisCid *cid.Cid
}

func init() {
	gopath, err := th.GetGoPath()
	if err != nil {
		panic(err)
	}

	detailspath := filepath.Join(gopath, "/src/github.com/filecoin-project/go-filecoin/fixtures/gen.json")
	detailsFile, err := os.Open(detailspath)
	defer func() {
		if err := detailsFile.Close(); err != nil {
			panic(err)
		}
	}()
	if err != nil {
		panic(err)
	}
	detailsFileBytes, err := ioutil.ReadAll(detailsFile)
	if err != nil {
		panic(err)
	}
	var details detailsStruct
	if err := json.Unmarshal(detailsFileBytes, &details); err != nil {
		panic(err)
	}

	var keys []string
	for key := range details.Keys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	miners := details.Miners

	for _, key := range keys {
		info := details.Keys[key]
		addr, err := info.Address()
		if err != nil {
			panic(err)
		}
		TestAddresses = append(TestAddresses, addr.String())
		testKeys = append(testKeys, fmt.Sprintf("%s.key", key))
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
