package internal_test

import (
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

	t.Run("returns error when migration step fails", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		runner.MigrationsProvider = testProviderMigrationFails
		assert.EqualError(t, runner.Run(), "migration failed: migration has failed")
	})

	t.Run("returns error when validation step fails", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		runner.MigrationsProvider = testProviderValidationFails
		assert.EqualError(t, runner.Run(), "validation failed: validation has failed")
	})

	t.Run("writes the new version to the repo", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		runner.MigrationsProvider = testProviderPasses

		assert.NoError(t, runner.Run())

		version, err := runner.GetNewRepoVersion()
		require.NoError(t, err)
		assert.Equal(t, uint(1), version)
	})
}
