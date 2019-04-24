package internal_test

import (
	"os"
	"testing"

	"github.com/golangci/golangci-lint/pkg/fsutils"
	ast "github.com/stretchr/testify/assert"
	req "github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestRepoMigrationHelper_GetOldRepo(t *testing.T) {
	tf.UnitTest(t)

	assert := ast.New(t)
	require := req.New(t)

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

	t.Run("Creates the dir", func(t *testing.T) {
		dirname := "/tmp/myfilecoindir"
		mustMakeTmpDir(require, dirname)
		defer mustRmDir(require, dirname)

		rmh := NewRepoMigrationHelper(dirname, "", "1", "2")
		or, err := rmh.MakeNewRepo()
		require.NoError(err)
		defer mustRmDir(require, or.Name())
		assert.True(fsutils.IsDir(or.Name()))
	})

}

func mustMakeTmpDir(require *req.Assertions, dirname string) {
	require.NoError(os.Mkdir(dirname, os.ModeDir|os.ModeTemporary|os.ModePerm))
}

// ensure that the error condition is checked when we clean up after creating tmpdirs.
func mustRmDir(require *req.Assertions, dirname string) {
	require.NoError(os.Remove(dirname))
}
