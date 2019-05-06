package internal

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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

// RequireRemove ensures that the error condition is checked when we clean up
// after creating a temporary directory.
func RequireRemove(t *testing.T, path string) {
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
func RequireSetupTestRepo(t *testing.T, repoVersion int) (repoDir, symLink string) {
	repoDir = RequireMakeTempDir(t, "testrepo")
	require.NoError(t, repo.InitFSRepo(repoDir, config.NewDefaultConfig()))

	symLink = repoDir + "-reposymlink"
	require.NoError(t, os.Symlink(repoDir, symLink))

	RequireSetRepoVersion(t, strconv.Itoa(repoVersion), repoDir)
	return repoDir, symLink
}

// RequireSetRepoVersion sets the version for the given test repo.
// This is here so that corrupt version strings can be tested.
func RequireSetRepoVersion(t *testing.T, repoVersion string, repoDir string) {
	verFile := repo.VersionFilename()
	newVer := []byte(repoVersion)
	require.NoError(t, ioutil.WriteFile(filepath.Join(repoDir, verFile), newVer, 0644))
}

// CaptureOutput puts log content into
func CaptureOutput(f func()) string {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	f()
	log.SetOutput(os.Stderr)
	return buf.String()
}
