package fortest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/filecoin-project/go-address"
	cid "github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/build/project"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	gen "github.com/filecoin-project/venus/tools/gengen/util"
)

// The file used to build these addresses can be found in:
// $GOPATH/src/github.com/filecoin-project/venus/fixtures/setup.json
//
// If said file is modified these addresses will need to change as well
// rebuild using
// TODO: move to build script
// https://github.com/filecoin-project/venus/issues/921
// cat ./fixtures/setup.json | ./tools/gengen/gengen --json --keypath fixtures > fixtures/genesis.car 2> fixtures/gen.json
//
// The fake cids used for commX in setup.json are generated using this tool:
// $GOPATH/src/github.com/filecoin-project/venus/tools/gengen/gencfg

// TestAddresses is a list of pregenerated addresses.
var TestAddresses []address.Address

// testKeys is a list of filenames, which contain the private keys of the pregenerated addresses.
var testKeys []string

// TestMiners is a list of pregenerated miner acccounts. They are owned by the matching TestAddress.
var TestMiners []address.Address

// TestGenGenConfig is the gengen config used to make the default test genesis block.
var TestGenGenConfig gen.GenesisCfg

type detailsStruct struct {
	Keys   []*crypto.KeyInfo
	Miners []struct {
		Owner               int
		Address             address.Address
		NumCommittedSectors uint64
	}
	GenesisCid cid.Cid
}

func init() {
	root := project.Root()

	genConfigPath := filepath.Join(root, "fixtures/setup.json")
	genConfigFile, err := os.Open(genConfigPath)
	if err != nil {
		return
	}
	defer func() {
		if err := genConfigFile.Close(); err != nil {
			panic(err)
		}
	}()
	genConfigBytes, err := ioutil.ReadAll(genConfigFile)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(genConfigBytes, &TestGenGenConfig)
	if err != nil {
		panic(err)
	}
}

func init() {
	root := project.Root()

	detailspath := filepath.Join(root, "fixtures/test/gen.json")
	detailsFile, err := os.Open(detailspath)
	if err != nil {
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
		TestAddresses = append(TestAddresses, addr)
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
