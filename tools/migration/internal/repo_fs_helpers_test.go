package internal_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestRepoMigrationHelper_CloneRepo(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Creates the dir with the right permissions", func(t *testing.T) {
		oldRepo := repo.RequireMakeTempDir(t, "")
		defer repo.RequireRemoveAll(t, oldRepo)

		linkedRepoPath := oldRepo + "something"
		require.NoError(t, os.Symlink(oldRepo, oldRepo+"something"))
		defer repo.RequireRemoveAll(t, linkedRepoPath)

		newRepoPath, err := CloneRepo(linkedRepoPath, 42)
		require.NoError(t, err)
		defer repo.RequireRemoveAll(t, newRepoPath)

		stat, err := os.Stat(newRepoPath)
		require.NoError(t, err)
		expectedPerms := "drwxr--r--"
		assert.Equal(t, expectedPerms, stat.Mode().String())
	})

	t.Run("fails if the old repo does not point to a symbolic link", func(t *testing.T) {
		oldRepo := repo.RequireMakeTempDir(t, "")
		defer repo.RequireRemoveAll(t, oldRepo)

		result, err := CloneRepo(oldRepo, 42)
		assert.Error(t, err, "old-repo must be a symbolic link.")
		assert.Equal(t, "", result)

		linkedRepoPath := oldRepo + "something"
		require.NoError(t, os.Symlink(oldRepo, oldRepo+"something"))
		defer repo.RequireRemoveAll(t, linkedRepoPath)

		result, err = CloneRepo(linkedRepoPath, 42)
		assert.NoError(t, err)
		assert.NotEqual(t, "", result)
	})

	t.Run("Increments the int on the end until a free filename is found", func(t *testing.T) {
		oldRepo := repo.RequireMakeTempDir(t, "")
		defer repo.RequireRemoveAll(t, oldRepo)

		linkedRepoPath := oldRepo + "something"
		require.NoError(t, os.Symlink(oldRepo, oldRepo+"something"))
		defer repo.RequireRemoveAll(t, linkedRepoPath)

		// Call CloneRepo several times and ensure that the filename end
		// is incremented, since these calls will happen in <1s.
		var paths []string
		var endRegex string
		for i := 0; i < 10; i++ {
			result, err := CloneRepo(linkedRepoPath, 42)
			require.NoError(t, err)

			// Presence of the trailing uniqueifier depends on whether the timestamp ticks over
			// a 1-second boundary.
			endRegex = "[0-9]{8}-[0-9]{6}-v042(-\\d+)?$"
			regx, err := regexp.Compile(endRegex)
			assert.NoError(t, err)
			assert.Regexp(t, regx, result)

			if len(paths) > 0 {
				assert.NotEqual(t, paths[len(paths)-1], result)
			}
			paths = append(paths, result)
		}
		for _, dir := range paths {
			repo.RequireRemoveAll(t, dir)
		}
	})
}

func TestRepoFSHelpers_InstallNewRepo(t *testing.T) {
	tf.UnitTest(t)

	t.Run("swaps out the symlink", func(t *testing.T) {
		container, repoLink := RequireInitRepo(t, 0)
		defer repo.RequireRemoveAll(t, container)

		oldRepoPath := repo.RequireReadLink(t, repoLink)
		newRepoPath, err := CloneRepo(repoLink, 42)
		require.NoError(t, err)

		require.NoError(t, InstallNewRepo(repoLink, newRepoPath))
		AssertNewRepoInstalled(t, newRepoPath, oldRepoPath, repoLink)
	})

	t.Run("returns error and leaves symlink if new repo does not exist", func(t *testing.T) {
		container, repoLink := RequireInitRepo(t, 0)
		defer repo.RequireRemoveAll(t, container)

		oldRepoPath := repo.RequireReadLink(t, repoLink)
		err := InstallNewRepo(repoLink, "/tmp/nonexistentfile")
		assert.EqualError(t, err, "stat /tmp/nonexistentfile: no such file or directory")

		assert.NotEqual(t, "/tmp/nonexistentfile", repo.RequireReadLink(t, repoLink))
		assert.Equal(t, oldRepoPath, repo.RequireReadLink(t, repoLink))
	})
}
