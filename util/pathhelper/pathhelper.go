package pathhelper

import (
	"log"
	"os"
	"os/user"
	"path/filepath"
)

// GenesisFilePath returns the path of the WalletFile
func GenesisFilePath() string {
	return ProjectRoot("/fixtures/genesis.car")
}

// ProjectRoot return the project root joined with any path fragments
func ProjectRoot(paths ...string) string {
	gopath, err := GetGoPath()
	if err != nil {
		panic(err)
	}

	allPaths := append([]string{gopath, "/src/github.com/filecoin-project/go-filecoin"}, paths...)

	return filepath.Join(allPaths...)
}

// GetGoPath returns the current go path for the user.
func GetGoPath() (string, error) {
	gp := os.Getenv("GOPATH")
	if gp != "" {
		return gp, nil
	}

	home, err := homedir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "go"), nil
}

func homedir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	return usr.HomeDir, nil
}
