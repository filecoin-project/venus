package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestMigrationRunner_RunInstall(t *testing.T) {
	tf.IntegrationTest(t)

	repoDir, repoSymlink := RequireSetupTestRepo(t, 0)
	defer RequireRemove(t, repoDir)
	defer RequireRemove(t, repoSymlink)

	t.Run("install happy path", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner := NewMigrationRunner(logger, "install", repoSymlink, "")
		runner.MigrationsProvider = testProviderPasses

	})

	t.Run("install returns error if new-repo option is not given", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner := NewMigrationRunner(logger, "install", repoSymlink, "")
		runner.MigrationsProvider = testProviderPasses

		assert.EqualError(t, runner.Run(), "installation failed: new repo is missing")
	})

	t.Run("install returns error if new-repo is not found", func(t *testing.T) {
		dummyLogFile, dummyLogPath := RequireOpenTempFile(t, "logfile")
		defer RequireRemove(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner := NewMigrationRunner(logger, "install", repoSymlink, "/tmp/nonexistent")
		runner.MigrationsProvider = testProviderPasses

		assert.EqualError(t, runner.Run(), "validation failed: open /tmp/nonexistent/version: no such file or directory")
	})
}
