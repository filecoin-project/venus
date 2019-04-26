package internal_test

import (
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/repo"
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	"github.com/golangci/golangci-lint/pkg/fsutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestRepoMigrationHelper_GetOldRepo(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Uses the option values when passed to ctor", func(t *testing.T) {
		dirname := mustMakeTmpDir(t, "filecoindir")
		defer mustRmDir(t, dirname)

		rmh := NewRepoFSWrangler(dirname, "", "1", "2")
		or, err := rmh.GetOldRepo()
		require.NoError(t, err)

		assert.Equal(t, dirname, or.Name())
	})
}

func TestRepoMigrationHelper_MakeNewRepo(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Creates the dir", func(t *testing.T) {
		dirname := "myfilecoindir"
		mustMakeTmpDir(t, dirname)
		defer mustRmDir(t, dirname)

		rmh := NewRepoFSWrangler(dirname, "", "1", "2")
		or, err := rmh.MakeNewRepo()
		require.NoError(t, err)
		defer mustRmDir(t, or.Name())
		assert.True(t, fsutils.IsDir(or.Name()))
	})

}

func TestGetNewRepoPath(t *testing.T) {
	tf.UnitTest(t)

	dirname := "myfilecoindir"

	t.Run("Uses the new repo opt as a prefix if provided", func(t *testing.T) {
		rmh := NewRepoFSWrangler(dirname, "somethingelse", "1", "2")
		newpath := rmh.GetNewRepoPath()
		rgx, err := regexp.Compile("somethingelse_1_2_[0-9]{8}-[0-9]{6}$")
		require.NoError(t, err)
		assert.Regexp(t, rgx, newpath)
	})

	t.Run("Adds a timestamp to the new repo dir", func(t *testing.T) {
		rmh := NewRepoFSWrangler(dirname, "", "1", "2")
		newpath := rmh.GetNewRepoPath()
		rgx, err := regexp.Compile("myfilecoindir_1_2_[0-9]{8}-[0-9]{6}$")
		require.NoError(t, err)
		assert.Regexp(t, rgx, newpath)
	})
}

func TestRepoFSWrangler_CopyRepo(t *testing.T) {
	tf.UnitTest(t)

	dirname := mustMakeTmpDir(t, "myfilecoindir")
	defer mustRmDir(t, dirname)
	require.NoError(t, repo.InitFSRepo(dirname, config.NewDefaultConfig()))

	wrangler := NewRepoFSWrangler(dirname, "", "1", "2")
	_, err  := wrangler.MakeNewRepo()
	require.NoError(t, err)
	require.NoError(t, wrangler.CopyRepo())
}

func mustMakeTmpDir(t *testing.T, prefix string) string {
	path, err := ioutil.TempDir("", prefix)
	require.NoError(t, err)
	return path
}

// ensure that the error condition is checked when we clean up after creating tmpdirs.
func mustRmDir(t *testing.T, dirname string) {
	require.NoError(t, os.RemoveAll(dirname))
}
