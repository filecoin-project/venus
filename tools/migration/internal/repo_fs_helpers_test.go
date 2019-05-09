package internal_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestRepoMigrationHelper_CloneRepo(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Creates the dir with the right permissions", func(t *testing.T) {
		oldRepo := RequireMakeTempDir(t, "")
		defer RequireRemoveAll(t, oldRepo)

		linkedRepoPath := oldRepo + "something"
		require.NoError(t, os.Symlink(oldRepo, oldRepo+"something"))
		defer RequireRemoveAll(t, linkedRepoPath)

		newRepoPath, err := CloneRepo(linkedRepoPath)
		require.NoError(t, err)
		defer RequireRemoveAll(t, newRepoPath)

		stat, err := os.Stat(newRepoPath)
		require.NoError(t, err)
		expectedPerms := "drwxr--r--"
		assert.Equal(t, expectedPerms, stat.Mode().String())
	})

	t.Run("fails if the old repo does not point to a symbolic link", func(t *testing.T) {
		oldRepo := RequireMakeTempDir(t, "")
		defer RequireRemoveAll(t, oldRepo)

		result, err := CloneRepo(oldRepo)
		assert.Error(t, err, "old-repo must be a symbolic link.")
		assert.Equal(t, "", result)

		linkedRepoPath := oldRepo + "something"
		require.NoError(t, os.Symlink(oldRepo, oldRepo+"something"))
		defer RequireRemoveAll(t, linkedRepoPath)

		result, err = CloneRepo(linkedRepoPath)
		assert.NoError(t, err)
		assert.NotEqual(t, "", result)
	})

	t.Run("Increments the int on the end until a free filename is found", func(t *testing.T) {
		oldRepo := RequireMakeTempDir(t, "")
		defer RequireRemoveAll(t, oldRepo)

		linkedRepoPath := oldRepo + "something"
		require.NoError(t, os.Symlink(oldRepo, oldRepo+"something"))
		defer RequireRemoveAll(t, linkedRepoPath)

		// Call CloneRepo several times and ensure that the filename end
		// is incremented, since these calls will happen in <1s.
		// 1000 times is more than enough; change this loop to 1000 and
		// this test fails because the index restarts, due to the timestamp
		// updating, which is correct behavior. Programmatically proving it restarts
		// in this test was more trouble than it was worth.
		var repos []string
		for i := 1; i < 5; i++ {
			result, err := CloneRepo(linkedRepoPath)
			require.NoError(t, err)
			repos = append(repos, result)
			endRegex := fmt.Sprintf("-%03d$", i)
			regx, err := regexp.Compile(endRegex)
			assert.NoError(t, err)
			assert.Regexp(t, regx, result)
		}
		for _, dir := range repos {
			RequireRemoveAll(t, dir)
		}

	})
}

func TestRepoFSHelpers_InstallNewRepo(t *testing.T) {
	tf.UnitTest(t)

	t.Run("swaps out the symlink", func(t *testing.T) {
		oldRepo, linkedRepoPath := RequireSetupTestRepo(t, 0)
		defer RequireRemoveAll(t, linkedRepoPath)
		defer RequireRemoveAll(t, oldRepo)

		newRepoPath, err := CloneRepo(linkedRepoPath)
		require.NoError(t, err)
		require.NoError(t, InstallNewRepo(linkedRepoPath, newRepoPath))

		AssertInstalled(t, newRepoPath, oldRepo, linkedRepoPath)
	})

	t.Run("returns error and leaves symlink if new repo does not exist", func(t *testing.T) {
		oldRepo, linkedRepoPath := RequireSetupTestRepo(t, 0)
		defer RequireRemoveAll(t, linkedRepoPath)
		defer RequireRemoveAll(t, oldRepo)

		_, err := CloneRepo(linkedRepoPath)
		require.NoError(t, err)

		err = InstallNewRepo(linkedRepoPath, "/tmp/nonexistentfile")
		assert.EqualError(t, err, "stat /tmp/nonexistentfile: no such file or directory")
		AssertNotInstalled(t, oldRepo, linkedRepoPath)
	})
}
