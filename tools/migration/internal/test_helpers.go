package internal

import (
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/repo"
)

// RequireMakeTempDir ensures that a temporary directory is created
func RequireMakeTempDir(t *testing.T, dirname string) string {
	newdir, err := ioutil.TempDir("", dirname)
	require.NoError(t, err)
	return newdir
}

// RequireRemoveAll ensures that the error condition is checked when we clean up
// after creating a temporary directory.
func RequireRemoveAll(t *testing.T, path string) {
	require.NoError(t, os.RemoveAll(path))
}

// RequireOpenTempFile is a shortcut for opening a given temp file with a given
// suffix, then returning both a filename and a file pointer.
func RequireOpenTempFile(t *testing.T, suffix string) (*os.File, string) {
	file, err := ioutil.TempFile("", suffix)
	require.NoError(t, err)
	name := file.Name()
	return file, name
}

// AssertLogged asserts that a given string is contained in the given log file.
func AssertLogged(t *testing.T, logFile *os.File, subStr string) {
	out, err := ioutil.ReadFile(logFile.Name())
	require.NoError(t, err)
	outStr := string(out)
	assert.Contains(t, outStr, subStr)
}

// RequireSetupTestRepo sets up a repo dir with a symlink pointing to it.
// Caller is responsible for deleting dir and symlink.
func RequireSetupTestRepo(t *testing.T, repoVersion uint) (repoDir, symLink string) {
	repoDir = RequireMakeTempDir(t, "testrepo")
	require.NoError(t, repo.InitFSRepo(repoDir, config.NewDefaultConfig()))

	symLink = repoDir + "-reposymlink"
	require.NoError(t, os.Symlink(repoDir, symLink))

	require.NoError(t, repo.WriteVersion(repoDir, repoVersion))
	return repoDir, symLink
}

// AssertNotInstalled verifies that repoLink still points to oldRepoDir
func AssertNotInstalled(t *testing.T, oldRepoDir, repoLink string) {
	newRepoTarget, err := os.Readlink(repoLink)
	require.NoError(t, err)
	assert.Equal(t, newRepoTarget, oldRepoDir)
}

// AssertInstalled verifies that the repoLink points to newRepoDir, and that
// oldRepoDir is still there
func AssertInstalled(t *testing.T, newRepoDir, oldRepoDir, repoLink string) {
	newRepoTarget, err := os.Readlink(repoLink)
	require.NoError(t, err)
	assert.Equal(t, newRepoTarget, newRepoDir)
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
