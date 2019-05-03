package internal_test

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestMigrationRunner_RunMigrate(t *testing.T) {
	tf.UnitTest(t)

	repoDir, repoSymlink := RequireSetupTestRepo(t, 0)
	defer RequireRemove(t, repoDir)
	defer RequireRemove(t, repoSymlink)

	t.Run("Returns error when Migration fails", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		runner.MigrationsProvider = testProviderMigrationFails
		assert.EqualError(t, runner.Run(), "migration failed: migration has failed")
	})

	t.Run("Returns error when Validation fails", func(t *testing.T) {
		dummyLogFile, err := ioutil.TempFile("", "logfile")
		require.NoError(t, err)
		logger := NewLogger(dummyLogFile, false)
		defer RequireRemove(t, dummyLogFile.Name())
		runner := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		runner.MigrationsProvider = testProviderValidationFails
		assert.EqualError(t, runner.Run(), "validation failed: validation has failed")
	})

	t.Run("Subsequent calls to Migrate migrate subsequent migrations", func(t *testing.T) {

	})

	t.Run("successful migration writes the new version to the repo", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		runner.MigrationsProvider = testProviderPasses

		migrations := runner.MigrationsProvider()
		assert.NotEmpty(t, migrations)
		assert.NoError(t, runner.Run())

	})
}
