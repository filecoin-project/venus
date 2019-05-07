package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestMigrationRunner_RunInstall(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("install swaps out symlink", func(t *testing.T) {
		repoDir, repoSymlink := RequireSetupTestRepo(t, 0)
		defer RequireRemove(t, repoDir)
		defer RequireRemove(t, repoSymlink)

		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)

		migratedDir, symlink := RequireSetupTestRepo(t, 1)
		RequireRemove(t, symlink) // don't need it
		defer RequireRemove(t, migratedDir)

		runner, err := NewMigrationRunner(logger, "install", repoSymlink, migratedDir)
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()
		AssertInstalled(t, runResult.NewRepoPath, repoDir, repoSymlink)
	})

	t.Run("install returns error if new-repo option is not given", func(t *testing.T) {
		repoDir, repoSymlink := RequireSetupTestRepo(t, 0)
		defer RequireRemove(t, repoDir)
		defer RequireRemove(t, repoSymlink)

		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "install", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()
		assert.EqualError(t, runResult.Err, "installation failed: new repo is missing")
	})

	t.Run("install returns error if new-repo is not found and does not remove symlink", func(t *testing.T) {
		repoDir, repoSymlink := RequireSetupTestRepo(t, 0)
		defer RequireRemove(t, repoDir)
		defer RequireRemove(t, repoSymlink)

		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "install", repoSymlink, "/tmp/nonexistent")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()

		assert.EqualError(t, runResult.Err, "installation failed: stat /tmp/nonexistent: no such file or directory")
		AssertNotInstalled(t, repoDir, repoSymlink)
	})

}
