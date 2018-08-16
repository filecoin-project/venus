package testhelpers

import (
	"path/filepath"
)

// The file used to build these addresses can be found in:
// $GOPATH/src/github.com/filecoin-project/go-filecoin/testhelpers/testfiles/setup.json
//
// If said file is modified these addresses will need to change as well
//

const TestAddress1 = "fcq8up4je9da37vf6dgs4jwytne435hdwxc6dyaas" // nolint: deadcode
const TestAddress2 = "fcq9gshrqyd0rk0xtylne6laxf4wfkge04gpahzpe" // nolint: deadcode
const TestAddress3 = "fcqcq3gtnukzq62pg7fm0he7nc84j6egkagdvqcks" // nolintb: deadcode
const TestAddress4 = "fcqyplzqvsvvlmev53yfxqp8688j4xkrcy8fnhz78" // nolint: deadcode, varcheck, megacheck
const TestAddress5 = "fcqmrw29j8l96naes25zrzkyydsaqnx5rdvfnv3lk" // nolint: deadcode, varcheck, megacheck

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
