package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestMigrationRunner_RunBuildonly(t *testing.T) {
	tf.UnitTest(t)

	repoDir, repoSymlink := RequireSetupTestRepo(t, 0)
	defer RequireRemove(t, repoDir)
	defer RequireRemove(t, repoSymlink)

	t.Run("clones repo, updates the version on success", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "buildonly", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		assert.NoError(t, runner.Run())

		version, err := runner.GetNewRepoVersion()
		require.NoError(t, err)
		assert.Equal(t, uint(1), version)
	})

	t.Run("writes the new version to the new repo, does not install", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "buildonly", repoSymlink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses
		assert.NoError(t, runner.Run())

		AssertBumpedVersion(t, runner.GetNewRepopath(), repoDir, 0)
		AssertNotInstalled(t, repoDir, repoSymlink)
	})
}
