package testhelpers

import (
	"path/filepath"
)

// The file used to build these addresses can be found in:
// $GOPATH/src/github.com/filecoin-project/go-filecoin/testhelpers/testfiles/setup.json
//
// If said file is modified these addresses will need to change as well
//

const TestAddress1 = "fcqv8pgdqxdk6x00shs5xzvr674al2nd8w5650ut0" // nolint: deadcode, golint
const TestKey1 = "a.key"                                         // nolint: deadcode, golint

const TestAddress2 = "fcqa9ux0ucsfyg7t0dmpe4t7jx3wml26598wfheaw" // nolint: deadcode, golint
const TestKey2 = "b.key"                                         // nolint: deadcode, golint

const TestAddress3 = "fcqyushu7qdtfkajzsvs7pk337m4rdjawmcu2kjp7" // nolint: deadcode, golint
const TestKey3 = "c.key"                                         // nolint: deadcode, golint

const TestAddress4 = "fcq2awe8astchttrjtsv7xtkczp6ee833mzdy42hr" // nolint: deadcode, varcheck, megacheck, golint
const TestKey4 = "d.key"                                         // nolint: deadcode, golint

const TestAddress5 = "fcq30ymjsn7pzrazg9yrwpr62wl3t7fvphqepuew6" // nolint: deadcode, varcheck, megacheck, golint
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
