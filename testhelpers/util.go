package testhelpers

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmdcULN1WCzgoQmcCaUAmEhwcxHYsDrbZ2LvRJKCL8dMrK/go-homedir"
)

// GetFreePort gets a free port from the kernel
// Credit: https://github.com/phayes/freeport
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close() // nolint: errcheck
	return l.Addr().(*net.TCPAddr).Port, nil
}

func getGoPath() (string, error) {
	gp := os.Getenv("GOPATH")
	if gp != "" {
		return gp, nil
	}

	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "go"), nil
}

//GetFilecoinBinary returns the path where the filecoin binary will be if it has been built.
func GetFilecoinBinary() (string, error) {
	gopath, err := getGoPath()
	if err != nil {
		return "", errors.Wrap(err, "failed to get GOPATH")
	}

	bin := filepath.Join(gopath, "/src/github.com/filecoin-project/go-filecoin/go-filecoin")
	_, err = os.Stat(bin)
	if err == nil {
		return bin, nil
	}

	if os.IsNotExist(err) {
		return "", fmt.Errorf("You are missing the filecoin binary...try building, searched in '%s'", bin)
	}

	return "", err
}
