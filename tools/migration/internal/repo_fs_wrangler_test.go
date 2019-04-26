package internal_test

import (
	"os"
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
		dirname := "/tmp/filecoindir"
		mustMakeTmpDir(t, dirname)
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
		dirname := "/tmp/myfilecoindir"
		mustMakeTmpDir(t, dirname)
		defer mustRmDir(t, dirname)

		rmh := NewRepoFSWrangler(dirname, "", "1", "2")
		or, err := rmh.MakeNewRepo()
		require.NoError(t, err)
		defer mustRmDir(t, or.Name())
		stat, err := or.Stat()
		require.NoError(t, err)
		assert.True(t, stat.IsDir())
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

func mustMakeTmpDir(t *testing.T, dirname string) {
	require.NoError(t, os.Mkdir(dirname, os.ModeDir|os.ModeTemporary|os.ModePerm))
}

// ensure that the error condition is checked when we clean up after creating tmpdirs.
func mustRmDir(t *testing.T, dirname string) {
	require.NoError(t, os.Remove(dirname))
}
