package testhelpers

import (
	"net"
	"os"
	"path/filepath"

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

// GetGoPath returns the current go path for the user.
func GetGoPath() (string, error) {
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

// MustGetFilecoinBinary returns the path where the filecoin binary will be if it has been built and panics otherwise.
func MustGetFilecoinBinary() string {
	gopath, err := GetGoPath()
	if err != nil {
		panic(err)
	}

	bin := filepath.Join(gopath, "/src/github.com/filecoin-project/go-filecoin/go-filecoin")
	_, err = os.Stat(bin)
	if err != nil {
		panic(err)
	}

	if os.IsNotExist(err) {
		panic("You are missing the filecoin binary...try building'")
	}

	return bin
}
