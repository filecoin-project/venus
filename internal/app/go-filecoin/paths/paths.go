package paths

import (
	"github.com/mitchellh/go-homedir"
	"os"
)

// node repo path defaults
const filPathVar = "FIL_PATH"
const defaultRepoDir = "~/.filecoin"

// GetRepoPath returns the path of the filecoin repo from a potential override
// string, the FIL_PATH environment variable and a default of ~/.filecoin/repo.
func GetRepoPath(override string) (string, error) {
	// override is first precedence
	if override != "" {
		return homedir.Expand(override)
	}
	// Environment variable is second precedence
	envRepoDir := os.Getenv(filPathVar)
	if envRepoDir != "" {
		return homedir.Expand(envRepoDir)
	}
	// Default is third precedence
	return homedir.Expand(defaultRepoDir)
}
