package testhelpers

import (
	"path/filepath"
)

// The file used to build these addresses can be found in:
// $GOPATH/src/github.com/filecoin-project/go-filecoin/testhelpers/testfiles/setup.json
//
// If said file is modified these addresses will need to change as well
//

const TestAddress1 = "fcqjq3pldz7ysy3fpg38rj87zqf7d7klx2mr0phc8" // nolint: deadcode, golint
const TestKey1 = "a.key"                                         // nolint: deadcode, golint

const TestAddress2 = "fcqa7alnxv28h7ecc8s52pmhwxguwfwum9m9t5ucl" // nolint: deadcode, golint
const TestKey2 = "b.key"                                         // nolint: deadcode, golint

const TestAddress3 = "fcq2fypy9gwfpa6s4accqr74twrkc8fhfp9ywxvuk" // nolint: deadcode, golint
const TestKey3 = "c.key"                                         // nolint: deadcode, golint

const TestAddress4 = "fcq054jpfs4lfy2ua5hzxa7rzg584uzympye6e3eq" // nolint: deadcode, varcheck, megacheck, golint
const TestKey4 = "d.key"                                         // nolint: deadcode, golint

const TestAddress5 = "fcqkss8dxwh3dx5kgp66rjyvasdsfpyqxpxg455ch" // nolint: deadcode, varcheck, megacheck, golint
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
