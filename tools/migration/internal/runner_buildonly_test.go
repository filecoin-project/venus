package internal_test

import (
	"os"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestMigrationRunner_RunBuildonly(t *testing.T) {
	tf.UnitTest(t)

	container, repoSymLink := RequireInitRepo(t, 0)
	oldRepoPath := repo.RequireReadLink(t, repoSymLink)
	defer repo.RequireRemoveAll(t, container)

	t.Run("clones repo, updates version, does not install", func(t *testing.T) {
		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "buildonly", repoSymLink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()
		assert.NoError(t, runResult.Err)

		stat, err := os.Stat(runResult.NewRepoPath)
		require.NoError(t, err)
		assert.True(t, stat.IsDir())

		AssertBumpedVersion(t, runResult.NewRepoPath, repoSymLink, 0)
		assert.Equal(t, uint(1), runResult.NewVersion)
		AssertLinked(t, oldRepoPath, repoSymLink)
	})
}
