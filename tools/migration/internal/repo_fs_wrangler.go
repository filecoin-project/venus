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

	newRepoPath := getNewRepoPath(oldRepoPath, "")

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

	archivedRepoPath := strings.Join([]string{realRepoPath, "archived", NowString()}, "_")

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
//     /Users/davonte/.filecoin_tmp_20190806-150455
func getNewRepoPath(oldPath, newRepoOpt string) string {
	var newRepoPrefix string
	if newRepoOpt != "" {
		newRepoPrefix = newRepoOpt
	} else {
		newRepoPrefix = oldPath
	}

	return strings.Join([]string{newRepoPrefix, "tmp", NowString()}, "_")
}
