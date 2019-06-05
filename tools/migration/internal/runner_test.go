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

	container, repoLink := RequireInitRepo(t, 0)
	oldRepoPath := repo.RequireReadLink(t, repoLink)
	defer repo.RequireRemoveAll(t, container)

	t.Run("valid command returns error if repo not found", func(t *testing.T) {
		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "describe", "/home/filecoin-symlink", "doesnt/matter")
		require.NoError(t, err)
		assert.Error(t, runner.Run().Err, "no filecoin repo found in /home/filecoin-symlink.")
	})

	t.Run("can set MigrationsProvider", func(t *testing.T) {
		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "describe", repoLink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		migrations := runner.MigrationsProvider()
		assert.NotEmpty(t, migrations)
		assert.NoError(t, runner.Run().Err)
	})

	t.Run("Does not run the migration if the repo is already up to date", func(t *testing.T) {
		require.NoError(t, repo.WriteVersion(oldRepoPath, 1))

		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "describe", repoLink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses
		assert.NoError(t, runner.Run().Err)
		AssertLogged(t, dummyLogFile, "Repo up-to-date: binary version 1 = repo version 1")
	})

	t.Run("Returns error when a valid migration is not found", func(t *testing.T) {
		require.NoError(t, repo.WriteVersion(oldRepoPath, 199))

		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "describe", repoLink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		assert.NoError(t, runner.Run().Err)

		out, err := ioutil.ReadFile(dummyLogPath)
		require.NoError(t, err)
		assert.Contains(t, string(out), "skipping migration from 0 to 1")
		assert.Contains(t, string(out), "did not find valid repo migration for version 199")
	})

	t.Run("Returns error when repo version is invalid", func(t *testing.T) {
		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "describe", repoLink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		require.NoError(t, ioutil.WriteFile(filepath.Join(oldRepoPath, "version"), []byte("-1"), 0644))
		assert.EqualError(t, runner.Run().Err, "repo version out of range: -1")

		require.NoError(t, repo.WriteVersion(oldRepoPath, 32767))
		assert.EqualError(t, runner.Run().Err, "repo version out of range: 32767")
	})

	t.Run("Returns error if version file does not contain an integer string", func(t *testing.T) {
		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "describe", repoLink, "")
		require.NoError(t, err)
		runner.MigrationsProvider = testProviderPasses

		require.NoError(t, ioutil.WriteFile(filepath.Join(oldRepoPath, "version"), []byte("foo"), 0644))
		assert.EqualError(t, runner.Run().Err, "repo version is corrupt: strconv.Atoi: parsing \"foo\": invalid syntax")
	})

	t.Run("describe does not clone repo", func(t *testing.T) {
		require.NoError(t, repo.WriteVersion(repoLink, 0))
		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "describe", repoLink, "")
		require.NoError(t, err)

		runner.MigrationsProvider = testProviderPasses

		runResult := runner.Run()
		require.NoError(t, runResult.Err)
		assert.Equal(t, "", runResult.NewRepoPath)
	})

	t.Run("run fails if there is more than 1 applicable migration", func(t *testing.T) {
		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "describe", repoLink, "")
		require.NoError(t, err)

		runner.MigrationsProvider = func() []Migration {
			return []Migration{
				&TestMigDoesNothing,
				&TestMigDoesNothing,
			}
		}
		assert.EqualError(t, runner.Run().Err, "migration check failed: found >1 migration for version 0; cannot proceed")
	})

	t.Run("run skips multiversion", func(t *testing.T) {
		dummyLogFile, dummyLogPath := repo.RequireOpenTempFile(t, "logfile")
		defer repo.RequireRemoveAll(t, dummyLogPath)
		logger := NewLogger(dummyLogFile, false)
		runner, err := NewMigrationRunner(logger, "describe", repoLink, "")
		require.NoError(t, err)

		runner.MigrationsProvider = func() []Migration {
			return []Migration{
				&TestMigDoesNothing,
				&TestMigMultiversion,
			}
		}
		assert.NoError(t, runner.Run().Err)

		out, err := ioutil.ReadFile(dummyLogPath)
		require.NoError(t, err)
		assert.Contains(t, string(out), "refusing multi-version migration from 1 to 3")
	})

	t.Run("newRepoOpt is ignored for commands other than install", func(t *testing.T) {
	})
}

func testProviderPasses() []Migration {
	return []Migration{&TestMigDoesNothing}
}

func testProviderValidationFails() []Migration {
	return []Migration{&TestMigFailsValidation}
}

func testProviderMigrationFails() []Migration {
	return []Migration{&TestMigFailsMigration}
}

type TestMigration struct {
	describeFunc func() string
	migrateFunc  func(string) error
	versionsFunc func() (uint, uint)
	validateFunc func(string, string) error
}

func (m *TestMigration) Describe() string {
	return m.describeFunc()
}

func (m *TestMigration) Migrate(newRepoPath string) error {
	return m.migrateFunc(newRepoPath)
}
func (m *TestMigration) Versions() (from, to uint) {
	return m.versionsFunc()
}

func (m *TestMigration) Validate(oldRepoPath, newRepoPath string) error {
	return m.validateFunc(oldRepoPath, newRepoPath)
}

var TestMigFailsValidation = TestMigration{
	describeFunc: func() string { return "migration fails validation step" },
	versionsFunc: func() (uint, uint) { return 0, 1 },
	migrateFunc:  func(string) error { return nil },
	validateFunc: func(string, string) error { return errors.New("validation has failed") },
}

var TestMigDoesNothing = TestMigration{
	describeFunc: func() string { return "the migration that doesn't do anything" },
	versionsFunc: func() (uint, uint) { return 0, 1 },
	migrateFunc:  func(string) error { return nil },
	validateFunc: func(string, string) error { return nil },
}

var TestMigFailsMigration = TestMigration{
	describeFunc: func() string { return "migration fails migration step" },
	versionsFunc: func() (uint, uint) { return 0, 1 },
	migrateFunc:  func(string) error { return errors.New("migration has failed") },
	validateFunc: func(string, string) error { return nil },
}

var TestMigMultiversion = TestMigration{
	describeFunc: func() string { return "the migration that skips a version" },
	versionsFunc: func() (uint, uint) { return 1, 3 },
	migrateFunc:  func(string) error { return nil },
	validateFunc: func(string, string) error { return nil },
}
