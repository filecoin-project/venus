package internal_test

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestMigrationRunner_RunInstall(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("swaps out symlink", func(t *testing.T) {
		container, repoSymlink := RequireInitRepo(t, 0)
		defer repo.RequireRemoveAll(t, container)

		oldPath := repo.RequireReadLink(t, repoSymlink)
		newPath, err := CloneRepo(repoSymlink, 1)
		require.NoError(t, err)
		require.NoError(t, repo.WriteVersion(newPath, 1)) // Fake migration

		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)

		runner, err := NewMigrationRunner(logger, "install", repoSymlink, newPath)
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()
		assert.NoError(t, runResult.Err)
		AssertNewRepoInstalled(t, runResult.NewRepoPath, oldPath, repoSymlink)
	})

	t.Run("returns error if new-repo option is not given", func(t *testing.T) {
		container, repoSymlink := RequireInitRepo(t, 0)
		defer repo.RequireRemoveAll(t, container)

		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)

		runner, err := NewMigrationRunner(logger, "install", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()
		assert.EqualError(t, runResult.Err, "installation failed: new repo path not specified")
	})

	t.Run("returns error if new-repo is not found, and does not remove symlink", func(t *testing.T) {
		container, repoSymlink := RequireInitRepo(t, 0)
		defer repo.RequireRemoveAll(t, container)
		oldPath := repo.RequireReadLink(t, repoSymlink)

		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "install", repoSymlink, "/tmp/nonexistent")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()

		assert.EqualError(t, runResult.Err, "installation failed: open /tmp/nonexistent/version: no such file or directory")
		AssertLinked(t, oldPath, repoSymlink)
	})

	t.Run("returns error if new repo does not have expected version", func(t *testing.T) {
		container, repoSymlink := RequireInitRepo(t, 0)
		defer repo.RequireRemoveAll(t, container)
		oldPath := repo.RequireReadLink(t, repoSymlink)

		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)

		newPath, err := CloneRepo(repoSymlink, 1)
		require.NoError(t, err)
		// Fail to actually update the version file

		runner, err := NewMigrationRunner(logger, "install", repoSymlink, newPath)
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()
		assert.EqualError(t, runResult.Err, "installation failed: repo has version 0, expected version 1")
		AssertLinked(t, oldPath, repoSymlink)
	})
}
