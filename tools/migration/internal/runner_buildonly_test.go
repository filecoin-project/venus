package internal_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestMigrationRunner_RunBuildonly(t *testing.T) {
	tf.UnitTest(t)

	repoDir, repoSymlink := RequireSetupTestRepo(t, 0)
	defer RequireRemoveAll(t, repoDir)
	defer RequireRemoveAll(t, repoSymlink)

	t.Run("clones repo", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "buildonly", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()
		assert.NoError(t, runResult.Err)
		stat, err := os.Stat(runResult.NewRepoPath)
		require.NoError(t, err)
		assert.True(t, stat.IsDir())
	})

	t.Run("writes the new version to the new repo, does not install", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "buildonly", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()
		assert.NoError(t, runResult.Err)

		AssertBumpedVersion(t, runResult.NewRepoPath, repoDir, 0)
		assert.Equal(t, uint(1), runResult.NewVersion)
		AssertNotInstalled(t, repoDir, repoSymlink)
	})
}
