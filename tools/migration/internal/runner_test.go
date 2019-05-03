package internal_test

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/repo"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestMigrationRunner_Run(t *testing.T) {
	tf.UnitTest(t)
	dummyLogFile, err := ioutil.TempFile("", "logfile")
	require.NoError(t, err)
	logger := NewLogger(dummyLogFile, false)
	defer func() {
		require.NoError(t, os.Remove(dummyLogFile.Name()))
	}()

	repoDir, repoSymlink := RequireSetupTestRepo(t, 0)
	defer RequireRmDir(t, repoDir)
	defer RequireRmDir(t, repoSymlink)

	t.Run("returns error if repo not found", func(t *testing.T) {
		runner := NewMigrationRunner(logger, "describe", "/home/filecoin-symlink")
		assert.Error(t, runner.Run(), "no filecoin repo found in /home/filecoin-symlink.")
	})

	t.Run("Can set MigrationsProvider", func(t *testing.T) {
		runner := NewMigrationRunner(false, "describe", repoSymlink)
		runner.MigrationsProvider = testProviderPasses

		migrations := runner.MigrationsProvider()
		assert.NotEmpty(t, migrations)
		assert.NoError(t, runner.Run())
	})

	t.Run("Returns error when Migration fails", func(t *testing.T) {
		runner := NewMigrationRunner(false, "migrate", repoSymlink)
		runner.MigrationsProvider = testProviderMigrationFails
		assert.EqualError(t, runner.Run(), "migration has failed")
	})

	t.Run("Returns error when Validation fails", func(t *testing.T) {
		runner := NewMigrationRunner(false, "migrate", repoSymlink)
		runner.MigrationsProvider = testProviderValidationFails
		assert.EqualError(t, runner.Run(), "validation has failed")
	})

	t.Run("Subsequent calls to Migrate migrate subsequent migrations", func(t *testing.T) {

	})

	t.Run("successful migration writes the new version to the repo", func(t *testing.T) {
		runner := NewMigrationRunner(false, "describe", repoSymlink)
		runner.MigrationsProvider = testProviderPasses

		migrations := runner.MigrationsProvider()
		assert.NotEmpty(t, migrations)
		assert.NoError(t, runner.Run())

	})

	t.Run("Returns error and does not not run the migration if the repo is already up to date", func(t *testing.T) {
		RequireSetRepoVersion(t, 1, repoDir)

		runner := NewMigrationRunner(false, "describe", repoSymlink)
		runner.MigrationsProvider = testProviderPasses
		assert.EqualError(t, runner.Run(), "binary version 1 = repo version 1; migration not run")
	})

	t.Run("Runs the right migration version", func(t *testing.T) {
		RequireSetRepoVersion(t, 1, repoDir)

		runner := NewMigrationRunner(false, "describe", repoSymlink)
		runner.MigrationsProvider = testProviderValidationFails
		assert.EqualError(t, runner.Run(), "binary version 1 = repo version 1; migration not run")
	})

	t.Run("Returns error when a valid migration is not found", func(t *testing.T) {
		RequireSetRepoVersion(t, 199, repoDir)

		runner := NewMigrationRunner(false, "describe", repoSymlink)
		runner.MigrationsProvider = testProviderPasses
		assert.EqualError(t, runner.Run(), "did not find valid repo migration for version 199")
	})

	t.Run("Returns error when repo version is invalid", func(t *testing.T) {
		runner := NewMigrationRunner(false, "describe", repoSymlink)
		runner.MigrationsProvider = testProviderPasses

		RequireSetRepoVersion(t, -1, repoDir)
		assert.EqualError(t, runner.Run(), "repo version out of range: -1")

		RequireSetRepoVersion(t, 32767, repoDir)
		assert.EqualError(t, runner.Run(), "repo version out of range: 32767")
	})

	t.Run("Returns error if version file does not contain an integer string", func(t *testing.T) {
		runner := NewMigrationRunner(false, "describe", repoSymlink)
		runner.MigrationsProvider = testProviderPasses

		// TODO: Handle this more gracefully
		require.NoError(t, ioutil.WriteFile(filepath.Join(repoDir, repo.VersionFilename()), []byte("foo"), 0644))
		assert.EqualError(t, runner.Run(), "strconv.Atoi: parsing \"foo\": invalid syntax")
	})
}

func testProviderPasses() []Migration {
	return []Migration{
		&TestMigration01{},
	}
}

func testProviderValidationFails() []Migration {
	return []Migration{&TestMigValidationFails{}}
}

func testProviderMigrationFails() []Migration {
	return []Migration{&TestMigMigrationFails{}}
}

type TestMigration01 struct {
}

func (m *TestMigration01) Describe() string {
	return "the migration that doesn't do anything"
}

func (m *TestMigration01) Migrate(newRepoPath string) error {
	return nil
}
func (m *TestMigration01) Versions() (from, to uint) {
	return 0, 1
}

func (m *TestMigration01) Validate(oldRepoPath, newRepoPath string) error {
	return nil
}

type TestMigMigrationFails struct {
}

func (m *TestMigMigrationFails) Versions() (from, to uint) {
	return 0, 1
}

func (m *TestMigMigrationFails) Describe() string {
	return "the migration that doesn't do anything"
}

func (m *TestMigMigrationFails) Migrate(newRepoPath string) error {
	return errors.New("migration has failed")
}

func (m *TestMigMigrationFails) Validate(oldRepoPath, newRepoPath string) error {
	return nil
}

type TestMigValidationFails struct {
}

func (m *TestMigValidationFails) Versions() (from, to uint) {
	return 0, 1
}

func (m *TestMigValidationFails) Describe() string {
	return "the migration that doesn't do anything"
}

func (m *TestMigValidationFails) Migrate(newRepoPath string) error {
	return nil
}

func (m *TestMigValidationFails) Validate(oldRepoPath, newRepoPath string) error {
	return errors.New("validation has failed")
}
