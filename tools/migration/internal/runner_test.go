package internal_test

import (
	"errors"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/repo"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

// TODO: Issue #2595 Implement first repo migration
func TestMigrationRunner_Run(t *testing.T) {
	tf.UnitTest(t)
	dummyLogFile, err := ioutil.TempFile("", "logfile")
	require.NoError(t, err)
	logger := NewLogger(dummyLogFile, false)
	defer func() {
		require.NoError(t, os.Remove(dummyLogFile.Name()))
	}()

	t.Run("returns error if repo not found", func(t *testing.T) {
		runner := NewMigrationRunner(logger, "describe", "/home/filecoin-symlink")
		assert.Error(t, runner.Run(), "no filecoin repo found in /home/filecoin-symlink.")
	})

	t.Run("Can set MigrationsProvider", func(t *testing.T) {

		repodir := RequireMakeTempDir(t, "testrepo")
		defer RequireRmDir(t, repodir)

		assert.NoError(t, repo.InitFSRepo(repodir, config.NewDefaultConfig()))

		// set version to 0.
		// Note this will break if the version file name changes
		assert.NoError(t, ioutil.WriteFile(filepath.Join(repodir, "version"), []byte("0"), 0644))

		runner := NewMigrationRunner(false, "describe", "/home/filecoin-symlink")
		runner.MigrationsProvider = testMigrationsProvider1
		migrations := runner.MigrationsProvider()
		assert.NotEmpty(t, migrations)

		err := runner.Run()
		assert.NoError(t, err)
	})
}

func testMigrationsProvider1() []Migration {
	return []Migration{
		&TestMigration01{},
		&TestMigMigrationFails{},
	}
}

func testMigrationsProvider2() []Migration {
	return []Migration{ &TestMigValidationFails{} }
}

type TestMigration01 struct {
}

func (m *TestMigration01) Describe() string {
	return "the migration that doesn't do anything"
}

func (m *TestMigration01) Migrate(newRepoPath string) error {
	return nil
}
func (m *TestMigration01) Versions() (from, to string) {
	return "0", "1"
}

func (m *TestMigration01) Validate(oldRepoPath, newRepoPath string) error {
	return nil
}

type TestMigMigrationFails struct {
}

func (m *TestMigMigrationFails) Versions() (from, to string) {
	return "1", "2"
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

func (m *TestMigValidationFails) Versions() (from, to string) {
	return "0", "1"
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
