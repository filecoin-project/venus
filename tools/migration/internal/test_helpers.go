package internal

import (
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/repo"
)

// AssertLogged asserts that a given string is contained in the given log file.
func AssertLogged(t *testing.T, logFile *os.File, subStr string) {
	out, err := ioutil.ReadFile(logFile.Name())
	require.NoError(t, err)
	outStr := string(out)
	assert.Contains(t, outStr, subStr)
}

// RequireInitRepo establishes a new repo symlink and directory inside a temporary container
// directory. Migrations of the repo are expected to be placed within the same container, such
// that a test can clean up arbitrary migrations by removing the container.
// Returns the path to the container directory, and the repo symlink inside it.
func RequireInitRepo(t *testing.T, version uint) (container, repoLink string) {
	container = repo.RequireMakeTempDir(t, "migration-test")
	repoLink = path.Join(container, "repo")
	err := repo.InitFSRepo(repoLink, version, config.NewDefaultConfig())
	require.NoError(t, err)
	return
}

// AssertLinked verifies that repoLink points to oldRepoDir
func AssertLinked(t *testing.T, repoDir, repoLink string) {
	newRepoTarget, err := os.Readlink(repoLink)
	require.NoError(t, err)
	assert.Equal(t, newRepoTarget, repoDir)
}

// AssertNewRepoInstalled verifies that the repoLink points to newRepoDir, and that
// oldRepoDir is still there
func AssertNewRepoInstalled(t *testing.T, newRepoDir, oldRepoDir, repoLink string) {
	linkTarget, err := os.Readlink(repoLink)
	require.NoError(t, err)
	assert.Equal(t, newRepoDir, linkTarget)
	oldRepoStat, err := os.Stat(oldRepoDir)
	require.NoError(t, err)
	assert.True(t, oldRepoStat.IsDir())
}

// AssertRepoVersion verifies that the version in repoPath is equal to versionStr
func AssertRepoVersion(t *testing.T, versionStr, repoPath string) {
	repoVersion, err := repo.ReadVersion(repoPath)
	require.NoError(t, err)
	assert.Equal(t, repoVersion, versionStr)
}

// AssertBumpedVersion checks that the version oldRepoDir is as expected,
// that the version in newRepoDir is updated by 1
func AssertBumpedVersion(t *testing.T, newRepoDir, oldRepoDir string, oldVersion uint64) {
	oldVersionStr := strconv.FormatUint(oldVersion, 10)
	AssertRepoVersion(t, oldVersionStr, oldRepoDir)
	newVersionStr := strconv.FormatUint(oldVersion+1, 10)
	AssertRepoVersion(t, newVersionStr, newRepoDir)
}
