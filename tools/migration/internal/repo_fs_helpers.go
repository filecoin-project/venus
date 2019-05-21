package internal

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	rcopy "github.com/otiai10/copy"
)

// This is a set of file system helpers for repo migration.
//
// CloneRepo and InstallRepo expect a symlink that points to the entire filecoin home
// directory, typically ~/.filecoin or whatever FIL_PATH is set to.
//
// This does touch sector data.

// CloneRepo copies the old repo to the new repo dir with Read/Write access.
//	 oldRepoLink must be a symlink. The symlink will be resolved and used for
//   copying.
//
//   The new repo dir name will look like: /Users/davonte/.filecoin-20190806-150455-001
//   If there is an existing dir by that name, the integer at the end will be
//   incremented until there is a free one or a new timestamp.
//
func CloneRepo(oldRepoLink string) (string, error) {
	realRepoPath, err := os.Readlink(oldRepoLink)
	if err != nil {
		return "", fmt.Errorf("old-repo must be a symbolic link: %s", err)
	}

	newRepoPath, err := makeNewRepoPath(oldRepoLink)
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
// returns any error.
func InstallNewRepo(oldRepoLink, newRepoPath string) error {
	if _, err := os.Readlink(oldRepoLink); err != nil {
		return err
	}

	if _, err := os.Stat(newRepoPath); err != nil {
		return err
	}

	if err := os.Remove(oldRepoLink); err != nil {
		return err
	}
	if err := os.Symlink(newRepoPath, oldRepoLink); err != nil {
		return err
	}
	return nil
}

// makeNewRepoPath generates a new repo path for a migration.
// Params:
//     oldPath:  the actual old repo path
// Returns:
//     a path generated using the above information plus tmp_<timestamp>.
//     error
// Example output:
//     /Users/davonte/.filecoin-20190806-150455-001
func makeNewRepoPath(oldPath string) (string, error) {
	// unlikely to see a name collision but make sure; making it loop up to 1000
	// ensures that even if there are 1000 calls/sec then the timestamp will change
	// anyway.
	var newpath string
	for i := 1; i < 1000; i++ {

		now := time.Now()
		nowStr := now.Format("20060102-150405")

		newpath = strings.Join([]string{oldPath, nowStr, fmt.Sprintf("%03d", i)}, "-")
		if _, err := os.Stat(newpath); os.IsNotExist(err) {
			return newpath, nil
		}
	}
	// this should never happen, but just in case.
	return "", errors.New("couldn't find a free dirname for cloning")
}
