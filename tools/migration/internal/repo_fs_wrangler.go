package internal

import (
	"errors"
	"fmt"
	"os"
	"strings"

	rcopy "github.com/otiai10/copy"
)

// CloneRepo copies the old repo to the new repo dir with Read/Write access.
//   Returns an error if the directory exists.
func CloneRepo(oldRepoPath string) (string, error) {
	realRepoPath, err := os.Readlink(oldRepoPath)
	if err != nil {
		return "", fmt.Errorf("old-repo must be a symbolic link: %s", err)
	}

	newRepoPath, err := getNewRepoPath(oldRepoPath, "")
	if err != nil {
		return "", err
	}

	if err := rcopy.Copy(realRepoPath, newRepoPath); err != nil {
		return "", err
	}
	if err := os.Chmod(newRepoPath, os.ModeDir|0744); err != nil {
		return "", err
	}
	return newRepoPath, nil
}

// InstallNewRepo archives the old repo, and symlinks the new repo in its place.
// returns the new path to the old repo and any error.
func InstallNewRepo(oldRepoLink, newRepoPath string) (string, error) {
	realRepoPath, err := os.Readlink(oldRepoLink)
	if err != nil {
		return "", err
	}

	archivedRepoPath := strings.Join([]string{realRepoPath, "archived", NowString()}, "-")

	if err := os.Rename(realRepoPath, archivedRepoPath); err != nil {
		return archivedRepoPath, err
	}
	if err := os.Remove(oldRepoLink); err != nil {
		return archivedRepoPath, err
	}
	if err := os.Symlink(newRepoPath, oldRepoLink); err != nil {
		return archivedRepoPath, err
	}
	return archivedRepoPath, nil
}

func OpenRepo(repoPath string) (*os.File, error) {
	return os.Open(repoPath)
}

// getNewRepoPath generates a new repo path for a migration.
// Params:
//     oldPath:  the actual old repo path
//     newRepoOpt:  whatever was passed in by the CLI (can be blank)
// Returns:
//     a path generated using the above information plus tmp_<timestamp>.
// Example output:
//     /Users/davonte/.filecoin_tmp_20190806-150455-001
func getNewRepoPath(oldPath, newRepoOpt string) (string, error) {
	var newRepoPrefix string
	if newRepoOpt != "" {
		newRepoPrefix = newRepoOpt
	} else {
		newRepoPrefix = oldPath
	}

	// unlikely to see a name collision but make sure; making it loop up to 1000
	// ensures that even if there are 1000 calls/sec then the timestamp will change
	// anyway.
	var newpath string
	for i := 1; i < 1000; i++ {
		newpath = strings.Join([]string{newRepoPrefix, NowString(), fmt.Sprintf("%03d", i)}, "-")
		if _, err := os.Stat(newpath); os.IsNotExist(err) {
			return newpath, nil
		}
	}
	// this should never happen, but just in case.
	return "", errors.New("couldn't find a free dirname for cloning")
}
