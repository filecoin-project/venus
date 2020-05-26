package paths

import (
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

// node repo path defaults
const filPathVar = "FIL_PATH"
const defaultRepoDir = "~/.filecoin"

// node sector storage path defaults
const filSectorPathVar = "FIL_SECTOR_PATH"
const defaultSectorDir = ".filecoin_sectors"
const defaultPieceStagingDir = "pieces"

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

// GetSectorPath returns the path of the filecoin sector storage from a
// potential override string, the FIL_SECTOR_PATH environment variable and a
// default of repoPath/../.filecoin_sectors.
func GetSectorPath(override, repoPath string) (string, error) {
	// override is first precedence
	if override != "" {
		return homedir.Expand(override)
	}
	// Environment variable is second precedence
	envRepoDir := os.Getenv(filSectorPathVar)
	if envRepoDir != "" {
		return homedir.Expand(envRepoDir)
	}
	// Default is third precedence: repoPath/../defaultSectorDir
	return homedir.Expand(filepath.Join(repoPath, "../", defaultSectorDir))
}

// PieceStagingDir returns the path to the piece staging directory repo path
func PieceStagingDir(repoPath string) (string, error) {
	return homedir.Expand(filepath.Join(repoPath, "../", defaultPieceStagingDir))
}
