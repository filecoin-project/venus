package internal

import (
	"os"
	"strings"
)

// RepoFSWrangler manages filesystem operations and figures out what the correct paths
// are for everything.
type RepoFSWrangler struct {
	oldRepoPath, newRepoPath string
}

// NewRepoFSWrangler takes options for old and new repo paths, figures out
// what the correct paths should be, and creates a new RepoFSWrangler with the
// correct paths.
func NewRepoFSWrangler(oldRepoOpt, newRepoPrefixOpt, oldVersion, newVersion string) *RepoFSWrangler {
	return &RepoFSWrangler{
		newRepoPath: getNewRepoPath(oldRepoOpt, newRepoPrefixOpt, oldVersion, newVersion),
		oldRepoPath: oldRepoOpt,
	}
}

// GetOldRepo returns the old repo dir, opened as read-only.
func (rmh *RepoFSWrangler) GetOldRepo() (*os.File, error) {
	return os.Open(rmh.oldRepoPath)
}

// GetOldRepoPath returns the path to the existing repo
func (rmh *RepoFSWrangler) GetOldRepoPath() string {
	return rmh.oldRepoPath
}

// GetNewRepoPath returns the path to the new repo. It makes no guarantees about
//   whether the directory exists.
func (rmh *RepoFSWrangler) GetNewRepoPath() string {
	return rmh.newRepoPath
}

// MakeNewRepo creates the new repo dir and returns it with Read/Write access.
//   This returns an error if the directory exists.
func (rmh *RepoFSWrangler) MakeNewRepo() (*os.File, error) {
	if err := os.Mkdir(rmh.newRepoPath, os.ModeDir|0744); err != nil {
		return nil, err
	}
	return os.Open(rmh.newRepoPath)
}

// getNewRepoPath generates a new repo path for a migration.
// Params:
//     oldPath:  the actual old repo path
//     newRepoOpt:  whatever was passed in by the CLI (can be blank)
//     oldVersion:  old repo version
//     newVersion:  version to migrate to
// Returns:
//     a path generated using all of the above information plus a timestamp.
// Example output:
//     /Users/davonte/.filecoin_1_2_20190806-150455
func getNewRepoPath(oldPath, newRepoOpt, oldVersion, newVersion string) string {
	var newRepoPrefix string
	if newRepoOpt != "" {
		newRepoPrefix = newRepoOpt
	} else {
		newRepoPrefix = oldPath
	}

	return strings.Join([]string{newRepoPrefix, oldVersion, newVersion, NowString()}, "_")
}
