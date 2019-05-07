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
		runner, err := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderMigrationFails
		assert.EqualError(t, runner.Run(), "migration failed: migration has failed")
	})

	t.Run("returns error when validation step fails", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderValidationFails
		assert.EqualError(t, runner.Run(), "validation failed: validation has failed")
	})

	t.Run("on success bumps version and installs new repo at symlink", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		assert.NoError(t, runner.Run())
		AssertBumpedVersion(t, runner.GetNewRepopath(), repoDir, 0)
		AssertInstalled(t, runner.GetNewRepopath(), repoDir, repoSymlink)
	})

}
