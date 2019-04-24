package internal

import (
	"os"
	"strings"

	"github.com/filecoin-project/go-filecoin/repo"
)

type RepoMigrationHelper struct {
	oldRepoPath, newRepoPath, oldVersion, newVersion string
}

// NewRepoMigrationHelper takes options for old and new repo paths, figures out
// what the correct paths should be, and creates a new RepoMigrationHelper with the
// correct paths.
func NewRepoMigrationHelper(oldRepoOpt, newRepoOpt, oldVersion, newVersion string) *RepoMigrationHelper {
	oldPath := GetOldRepoPath(oldRepoOpt)
	return &RepoMigrationHelper{
		newRepoPath: GetNewRepoPath(oldPath, newRepoOpt, oldVersion, newVersion),
		newVersion:  newVersion,
		oldRepoPath: oldPath,
		oldVersion:  oldVersion,
	}
}

// GetOldRepo returns the old repo dir, opened as read-only.
func (rmh *RepoMigrationHelper) GetOldRepo() (*os.File, error) {
	return os.Open(GetOldRepoPath(rmh.oldRepoPath))
}

// MakeNewRepo creates the new repo dir and returns it with Read/Write access.
func (rmh *RepoMigrationHelper) MakeNewRepo() (*os.File, error) {
	if err := os.Mkdir(rmh.newRepoPath, os.ModeDir|0744); err != nil {
		return nil, err
	}
	return os.Open(rmh.newRepoPath)
}

// GetOldRepoPath takes a command line option (which can be blank) and uses it
// to find the correct old repo path.
func GetOldRepoPath(cliOpt string) string {
	dirname := repo.GetRepoDir(cliOpt)
	return ExpandHomedir(dirname)
}

// GetNewRepoPath generates a new repo path for a migration.
// Params:
//     oldPath:  the actual old repo path
//  newRepoOpt:  whatever was passed in by the CLI (can be blank)
//  oldVersion:  old repo version
//  newVersion:  version to migrate to
// Returns:  a path generated using all of the above information plus a timestamp.
// Example output:   /Users/davonte/.filecoin_1_2_2019-08-06_150455
func GetNewRepoPath(oldPath, newRepoOpt, oldVersion, newVersion string) string {
	var newRepoDir string
	if newRepoOpt != "" {
		newRepoDir = newRepoOpt
	} else {
		newRepoDir = oldPath
	}

	return strings.Join([]string{newRepoDir, oldVersion, newVersion, NowString()}, "_")
}
