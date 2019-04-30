package internal_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestRepoMigrationHelper_CloneRepo(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Creates the dir with the right permissions", func(t *testing.T) {
		oldRepo := requireMakeTempDir(t, "")
		defer requireRmDir(t, oldRepo)

		linkedRepoPath := oldRepo + "something"
		require.NoError(t, os.Symlink(oldRepo, oldRepo+"something"))
		defer requireRmDir(t, linkedRepoPath)

		newRepoPath, err := CloneRepo(linkedRepoPath)
		require.NoError(t, err)
		defer requireRmDir(t, newRepoPath)

		stat, err := os.Stat(newRepoPath)
		require.NoError(t, err)
		expectedPerms := "drwxr--r--"
		assert.Equal(t, expectedPerms, stat.Mode().String())
	})

	t.Run("fails if the old repo does not point to a symbolic link", func(t *testing.T) {
		oldRepo := requireMakeTempDir(t, "")
		defer requireRmDir(t, oldRepo)

		result, err := CloneRepo(oldRepo)
		assert.Error(t, err, "old-repo must be a symbolic link.")
		assert.Equal(t, "", result)

		linkedRepoPath := oldRepo + "something"
		require.NoError(t, os.Symlink(oldRepo, oldRepo+"something"))
		defer requireRmDir(t, linkedRepoPath)

		result, err = CloneRepo(linkedRepoPath)
		assert.NoError(t, err)
		assert.NotEqual(t, "", result)
	})
}

func TestRepoFSWrangler_InstallNewRepo(t *testing.T) {
	tf.UnitTest(t)

	oldRepo := requireMakeTempDir(t, "")
	// put something in each repo dir so we know which is which
	_, err := os.Create(path.Join(oldRepo, "oldRepoFile"))
	require.NoError(t, err)

	linkedRepoPath := oldRepo + "something"
	require.NoError(t, os.Symlink(oldRepo, oldRepo+"something"))
	defer requireRmDir(t, linkedRepoPath)

	newRepoPath, err := CloneRepo(linkedRepoPath)
	require.NoError(t, err)

	// put something in each repo dir so we know which is which
	_, err = os.Create(path.Join(newRepoPath, "newRepoFile"))
	require.NoError(t, err)

	archivedRepo, err := InstallNewRepo(linkedRepoPath, newRepoPath)
	require.NoError(t, err)

	// check that the archive is there
	dir, err := os.Open(archivedRepo)
	require.NoError(t, err)
	contents, err := dir.Readdirnames(0)
	require.NoError(t, err)
	assert.Contains(t, contents, "oldRepoFile")

	// check that the new repo is at the old location.
	dir, err = os.Open(newRepoPath)
	require.NoError(t, err)
	contents, err = dir.Readdirnames(0)
	require.NoError(t, err)
	assert.Contains(t, contents, "newRepoFile")
}

func requireMakeTempDir(t *testing.T, dirname string) string {
	newdir, err := ioutil.TempDir("", dirname)
	require.NoError(t, err)
	return newdir
}

// ensure that the error condition is checked when we clean up after creating tmpdirs.
func requireRmDir(t *testing.T, dirname string) {
	require.NoError(t, os.RemoveAll(dirname))
}
