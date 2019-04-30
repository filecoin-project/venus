package internal

import (
	"os"
	"strings"

	rcopy "github.com/otiai10/copy"
)

// CloneRepo copies the old repo to the new repo dir with Read/Write access.
//   Returns an error if the directory exists.
func CloneRepo(oldRepoPath string) (string, error) {
	newRepoPath := getNewRepoPath(oldRepoPath, "")

	if err := rcopy.Copy(oldRepoPath, newRepoPath); err != nil {
		return "", err
	}
	if err := os.Chmod(newRepoPath, os.ModeDir|0744); err != nil {
		return "", err
	}
	return newRepoPath, nil
}

// InstallNewRepo archives the old repo, and symlinks the new repo in its place.
// returns the new path to the old repo and any error.
func InstallNewRepo(oldRepoPath, newRepoPath string) (string, error) {
	archivedRepo := strings.Join([]string{oldRepoPath, NowString()}, "-")

	if err := os.Rename(oldRepoPath, archivedRepo); err != nil {
		return archivedRepo, err
	}
	if err := os.Symlink(newRepoPath, oldRepoPath); err != nil {
		return archivedRepo, err
	}
	return archivedRepo, nil
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
