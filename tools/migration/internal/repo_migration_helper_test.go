package internal_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/golangci/golangci-lint/pkg/fsutils"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"

	ast "github.com/stretchr/testify/assert"
	req "github.com/stretchr/testify/require"
)

func TestRepoMigrationHelper_GetOldRepo(t *testing.T) {
	tf.UnitTest(t)

	assert := ast.New(t)
	require := req.New(t)

	t.Run("Returns with default values when ctor passed empty strings", func(t *testing.T) {
		rmh := NewRepoMigrationHelper("", "", "1", "2")
		or, err := rmh.GetOldRepo()
		require.NoError(err)

		rgx, err := regexp.Compile(".filecoin$")
		require.NoError(err)
		assert.Regexp(rgx, or.Name())
	})

	t.Run("Uses the option values when passed to ctor", func(t *testing.T) {
		dirname := "/tmp/filecoindir"
		mustMakeTmpDir(require, dirname)
		defer mustRmDir(require, dirname)

		rmh := NewRepoMigrationHelper(dirname, "", "1", "2")
		or, err := rmh.GetOldRepo()
		require.NoError(err)

		assert.Equal(dirname, or.Name())
	})
}

func TestRepoMigrationHelper_MakeNewRepo(t *testing.T) {
	tf.UnitTest(t)

	assert := ast.New(t)
	require := req.New(t)

	t.Run("Adds a timestamp to the new repo dir", func(t *testing.T) {
		dirname := "/tmp/myfilecoindir"
		mustMakeTmpDir(require, dirname)
		defer mustRmDir(require, dirname)

		rmh := NewRepoMigrationHelper(dirname, "", "1", "2")
		or, err := rmh.MakeNewRepo()
		require.NoError(err)
		defer mustRmDir(require, or.Name())

		rgx, err := regexp.Compile("/tmp/myfilecoindir_1_2_[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{6}")
		require.NoError(err)

		assert.Regexp(rgx, or.Name())
		assert.True(fsutils.IsDir(or.Name()))
	})

	t.Run("Uses the new repo opt as a prefix if provided", func(t *testing.T) {
		dirname := "/tmp/mynew_repodir"

		rmh := NewRepoMigrationHelper("", dirname, "1", "2")
		or, err := rmh.MakeNewRepo()
		require.NoError(err)
		defer mustRmDir(require, or.Name())

		rgx, err := regexp.Compile("/tmp/mynew_repodir_1_2_[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{6}")
		require.NoError(err)

		assert.Regexp(rgx, or.Name())
		assert.True(fsutils.IsDir(or.Name()))
	})

}

func TestGetOldRepoPath(t *testing.T) {
	// technically it'll return what's at FIL_PATH if that is set,
	// but in testing this is set to the default.
	tf.UnitTest(t)
	assert := ast.New(t)

	home := os.Getenv("HOME")
	expected := strings.Join([]string{home, "/.filecoin"}, "")
	assert.Equal(expected, GetOldRepoPath(""))
}

func TestMakeNewRepo(t *testing.T) {
	tf.UnitTest(t)
}

func mustMakeTmpDir(require *req.Assertions, dirname string) {
	require.NoError(os.Mkdir(dirname, os.ModeDir|os.ModeTemporary|os.ModePerm))
}

// ensure that the error condition is checked when we clean up after creating tmpdirs.
func mustRmDir(require *req.Assertions, dirname string) {
	require.NoError(os.Remove(dirname))
}
