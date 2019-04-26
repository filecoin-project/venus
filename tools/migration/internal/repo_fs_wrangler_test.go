package internal_test

import (
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestRepoMigrationHelper_GetOldRepo(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Uses the option values when passed to ctor", func(t *testing.T) {
		oldRepo := mustMakeTmpDir(t, "")
		defer mustRmDir(t, oldRepo)

		rmh := NewRepoFSWrangler(oldRepo, "", "1", "2")
		or, err := rmh.GetOldRepo()
		require.NoError(t, err)

		assert.Equal(t, oldRepo, or.Name())
	})
}

func TestRepoMigrationHelper_MakeNewRepo(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Creates the dir with the right permissions", func(t *testing.T) {
		oldRepo := mustMakeTmpDir(t, "")
		defer mustRmDir(t, oldRepo)

		rmh := NewRepoFSWrangler(oldRepo, "", "1", "2")
		require.NoError(t, rmh.MakeNewRepo())
		defer mustRmDir(t, rmh.GetNewRepoPath())

		stat, err := os.Stat(rmh.GetNewRepoPath())
		require.NoError(t, err)
		expectedPerms := "drwxr--r--"
		assert.Equal(t, expectedPerms, stat.Mode().String())
	})

}

func TestGetNewRepoPath(t *testing.T) {
	tf.UnitTest(t)

	dirname := "/tmp/myfilecoindir"

	t.Run("Uses the new repo opt as a prefix if provided", func(t *testing.T) {
		rmh := NewRepoFSWrangler(dirname, "/tmp/somethingelse", "1", "2")
		newpath := rmh.GetNewRepoPath()
		rgx, err := regexp.Compile("/tmp/somethingelse_1_2_[0-9]{8}-[0-9]{6}$")
		require.NoError(t, err)
		assert.Regexp(t, rgx, newpath)
	})

	t.Run("Adds a timestamp to the new repo dir", func(t *testing.T) {
		rmh := NewRepoFSWrangler(dirname, "", "1", "2")
		newpath := rmh.GetNewRepoPath()
		rgx, err := regexp.Compile("/tmp/myfilecoindir_1_2_[0-9]{8}-[0-9]{6}$")
		require.NoError(t, err)
		assert.Regexp(t, rgx, newpath)
	})
}

func TestRepoFSWrangler_MakeNewRepo(t *testing.T) {
	dirname := mustMakeTmpDir(t, "")
	rmh := NewRepoFSWrangler(dirname, "", "1", "2")
	require.NoError(t, rmh.MakeNewRepo())
	dir, err := os.Open(rmh.GetNewRepoPath())
	require.NoError(t, err)
	stat, err := dir.Stat()
	require.NoError(t, err)
	expectedPerms := "drwxr--r--"
	assert.Equal(t, expectedPerms, stat.Mode().String())
}

func TestRepoFSWrangler_InstallNewRepo(t *testing.T) {
	oldRepo := mustMakeTmpDir(t, "")
	rmh := NewRepoFSWrangler(oldRepo, "", "1", "2")
	// put something in each repo dir so we know which is which
	_, err := os.Create(path.Join(oldRepo, "oldRepoFile"))
	require.NoError(t, err)
	require.NoError(t, rmh.MakeNewRepo())
	_, err = os.Create(path.Join(rmh.GetNewRepoPath(), "newRepoFile"))
	require.NoError(t, err)

	archivedRepo, err := rmh.InstallNewRepo()
	require.NoError(t, err)

	// check that the archive is there
	dir, err := os.Open(archivedRepo)
	require.NoError(t, err)
	stat, err := dir.Stat()
	require.NoError(t, err)
	expectedPerms := "dr--r--r--"
	assert.Equal(t, expectedPerms, stat.Mode().String())
	contents, err := dir.Readdirnames(0)
	require.NoError(t, err)
	assert.Contains(t, contents, "oldRepoFile")

	// check that the new repo is at the old location.
	dir, err = os.Open(rmh.GetOldRepoPath())
	require.NoError(t, err)
	contents, err = dir.Readdirnames(0)
	require.NoError(t, err)
	assert.Contains(t, contents, "newRepoFile")
}

func mustMakeTmpDir(t *testing.T, dirname string) string {
	newdir, err := ioutil.TempDir("", dirname)
	require.NoError(t, err)
	return newdir
}

// ensure that the error condition is checked when we clean up after creating tmpdirs.
func mustRmDir(t *testing.T, dirname string) {
	require.NoError(t, os.RemoveAll(dirname))
}
