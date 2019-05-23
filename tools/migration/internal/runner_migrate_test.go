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
	defer RequireRemoveAll(t, repoDir)
	defer RequireRemoveAll(t, repoSymlink)

	t.Run("returns error when migration step fails", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderMigrationFails
		runResult := runner.Run()

		assert.EqualError(t, runResult.Err, "migration failed: migration has failed")
	})

	t.Run("returns error when validation step fails", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderValidationFails
		runResult := runner.Run()
		assert.EqualError(t, runResult.Err, "validation failed: validation has failed")
	})

	t.Run("on success bumps version and installs new repo at symlink", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "migrate", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()
		assert.NoError(t, runResult.Err)
		AssertBumpedVersion(t, runResult.NewRepoPath, repoDir, 0)
		AssertInstalled(t, runResult.NewRepoPath, repoDir, repoSymlink)
	})

}
