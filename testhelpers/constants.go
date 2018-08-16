package testhelpers

import (
	"path/filepath"
)

// The file used to build these addresses can be found in:
// $GOPATH/src/github.com/filecoin-project/go-filecoin/testhelpers/testfiles/setup.json
//
// If said file is modified these addresses will need to change as well
//

const TestAddress1 = "fcqat3u6nmev9sy52r5djnkq9qjdyg7j9vxt0ayud" // nolint: deadcode
const TestAddress2 = "fcq0dtn4s0dqpmevthqvz36xm7xftkkx4nnqsq8ku" // nolint: deadcode
const TestAddress3 = "fcqtruwed863w9vgz82tjw90erpj3t74yqzv4szwj" // nolint: deadcode
const TestAddress4 = "fcq2gz4yqx5dusqecxe79aa4kxhj8ejn645s67k9f" // nolint: deadcode, varcheck, megacheck
const TestAddress5 = "fcq5y8lp6s6jtdavp0nahyr4hpufjfedtjcrntuf"  // nolint: deadcode, varcheck, megacheck

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
