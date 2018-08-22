package testhelpers

import (
	"path/filepath"
)

// The file used to build these addresses can be found in:
// $GOPATH/src/github.com/filecoin-project/go-filecoin/testhelpers/testfiles/setup.json
//
// If said file is modified these addresses will need to change as well
//

const TestAddress1 = "fcqs9lxrshrn52yarn8qscnz7j2heyn4dm6shup3y" // nolint: deadcode, golint
const TestKey1 = "a.key"                                         // nolint: deadcode, golint

const TestAddress2 = "fcqdp5fl29cxp6sqgqdy5n5ugt9krmm79ezje9ata" // nolint: deadcode, golint
const TestKey2 = "b.key"                                         // nolint: deadcode, golint

const TestAddress3 = "fcqgcuefjvr30lc4rp9cr9u5pf3v6rgeln76kt8r8" // nolint: deadcode, golint
const TestKey3 = "c.key"                                         // nolint: deadcode, golint

const TestAddress4 = "fcqsnfdhlpyyfzakfupn43kzxdcc6nrk7ga39043y" // nolint: deadcode, varcheck, megacheck, golint
const TestKey4 = "d.key"                                         // nolint: deadcode, golint

const TestAddress5 = "fcqhlkgmuhmrhfdq7hfa7mlg6x692kutf0lve9sqn" // nolint: deadcode, varcheck, megacheck, golint
const TestKey5 = "e.key"                                         // nolint: deadcode, golint

// GenesisFilePath returns the path of the WalletFile
// Head after running with setup.json is: zdpuAkgCshuhMj8nB5nHW3HFboWpvz8JxKvHxMBfDCaKeV2np
func GenesisFilePath() string {
	gopath, err := getGoPath()
	if err != nil {
		panic(err)
	}

	return filepath.Join(gopath, "/src/github.com/filecoin-project/go-filecoin/testhelpers/testfiles/genesis.car")
}

// KeyFilePaths returns the paths to the wallets of the testaddresses
func KeyFilePaths() []string {
	gopath, err := getGoPath()
	if err != nil {
		panic(err)
	}
	folder := "/src/github.com/filecoin-project/go-filecoin/testhelpers/testfiles/"

	return []string{
		filepath.Join(gopath, folder, "a.key"),
		filepath.Join(gopath, folder, "b.key"),
		filepath.Join(gopath, folder, "c.key"),
		filepath.Join(gopath, folder, "d.key"),
		filepath.Join(gopath, folder, "e.key"),
	}
}

func keyFilePath(kf string) string {
	gopath, err := getGoPath()
	if err != nil {
		panic(err)
	}
	folder := "/src/github.com/filecoin-project/go-filecoin/testhelpers/testfiles/"

	return filepath.Join(gopath, folder, kf)
}
