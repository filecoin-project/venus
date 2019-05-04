package internal

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

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

// RequireSetupTestRepo sets up a repo dir with a symlink pointing to it.
// Caller is responsible for deleting dir and symlink.
func RequireSetupTestRepo(t *testing.T, repoVersion int) (repoDir, symLink string) {
	repoDir = RequireMakeTempDir(t, "testrepo")
	require.NoError(t, repo.InitFSRepo(repoDir, config.NewDefaultConfig()))

	symLink = repoDir + "-reposymlink"
	require.NoError(t, os.Symlink(repoDir, symLink))

	RequireSetRepoVersion(t, repoVersion, repoDir)
	return repoDir, symLink
}

// RequireSetRepoVersion sets the version for the given test repo.
// even though the version is uint, using int allows us to test with invalid repoVersions
// such as -1
func RequireSetRepoVersion(t *testing.T, repoVersion int, repoDir string) {
	verFile := repo.VersionFilename()
	newVer := []byte(fmt.Sprintf("%d", repoVersion))
	require.NoError(t, ioutil.WriteFile(filepath.Join(repoDir, verFile), newVer, 0644))
}
