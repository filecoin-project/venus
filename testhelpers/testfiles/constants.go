package testhelpers

import (
	"go/build"
	"os"
	"path/filepath"
)

// WalletFilePath returns the path of the WalletFile
func WalletFilePath() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	return filepath.Join(gopath, "/src/github.com/filecoin-project/go-filecoin/testhelpers/testfiles/walletGenFile.toml")
}
