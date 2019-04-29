package paths

import (
	"os"
	"path/filepath"
)

// node repo path defaults
const filPathVar = "FIL_PATH"
const defaultHomeDir = "~/.filecoin"
const defaultRepoDir = "repo"

// node sector storage path defaults
const filSectorPathVar = "FIL_SECTOR_PATH"
const defaultSectorDir = "sectors"
const defaultSectorStagingDir = "staging"
const defaultSectorSealingDir = "sealed"

// GetRepoPath returns the path of the filecoin repo from a potential override
// string, the FIL_PATH environment variable and a default of ~/.filecoin/repo.
func GetRepoPath(override string) string {
	// override is first precedence
	if override != "" {
		return override
	}
	// Environment variable is second precedence
	envRepoDir := os.Getenv(filPathVar)
	if envRepoDir != "" {
		return envRepoDir
	}
	// Default is third precedence
	return filepath.Join(defaultHomeDir, defaultRepoDir)
}

// GetSectorPath returns the path of the filecoin sector storage from a
// potential override string, the FIL_SECTOR_PATH environment variable and a
// default of ~/.filecoin/sectors.
func GetSectorPath(override string) string {
	// override is first precedence
	if override != "" {
		return override
	}
	// Environment variable is second precedence
	envRepoDir := os.Getenv(filSectorPathVar)
	if envRepoDir != "" {
		return envRepoDir
	}
	// Default is third precedence
	return filepath.Join(defaultHomeDir, defaultSectorDir)
}

// StagingDir returns the path to the sector staging directory given the sector
// storage directory path.
func StagingDir(sectorPath string) string {
	return filepath.Join(sectorPath, defaultSectorStagingDir)
}

// StagingDir returns the path to the sector sealed directory given the sector
// storage directory path.
func SealedDir(sectorPath string) string {
	return filepath.Join(sectorPath, defaultSectorSealingDir)
}
