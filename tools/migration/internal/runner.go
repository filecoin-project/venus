package internal

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/filecoin-project/go-filecoin/repo"
)

// Migration is the interface to all repo migration versions.
type Migration interface {
	// Describe returns a list of steps, as a formatted string, that a given Migration will take.
	// These should correspond to named functions in the given Migration.
	Describe() string

	// Migrate performs all migration steps for the Migration that implements the interface.
	// Migrate  expects newRepo to be:
	//		a directory
	// 		read/writeable by this process,
	//      contain a copy of the old repo.
	Migrate(newRepoPath string) error

	Versions() (from, to uint)

	// Validate performs validation operations, using oldRepo for comparison.
	// Validation requirements will be different for every migration.
	// Validate expects newRepo to be
	//		a directory
	//      readable by this process
	//      already migrated
	//  It expects oldRepo to be
	//		a directory
	//      read-only by this process
	//  A successful validation returns nil.
	Validate(oldRepoPath, newRepoPath string) error
}

type MigrationRunner struct {
	verbose            bool
	command            string
	oldRepoOpt         string
	MigrationsProvider func() []Migration
}

func NewMigrationRunner(verb bool, command, oldRepoOpt string) *MigrationRunner {
	return &MigrationRunner{
		verbose:    verb,
		command:    command,
		oldRepoOpt: oldRepoOpt,
	}
}

func (m *MigrationRunner) Run() error {
	// TODO: Issue #2595 Implement first repo migration
	// detect current version of old repo
	repoVersion, err := m.loadVersion()
	if err != nil {
		return err
	}
	if repoVersion == repo.Version {
		return errors.New("binary version = repo version; migration not run")
	}
	// iterate over migrations provided by the Provider.
	for _, mig := range m.MigrationsProvider() {
		from, to := mig.Versions()
		// look for a migration that upgrades the old repo by 1 version
		if from == repoVersion && to == from+1 {
			// if there is one, perform the migration command
			newRepoPath, err := getNewRepoPath(m.oldRepoOpt, "")

			if err != nil {
				return err
			}

			if err = mig.Migrate(newRepoPath); err != nil {
				return err
			}
			return nil
		}
	}
	// else exit with error
	return fmt.Errorf("did not find valid repo migration for version %d to version %d", repoVersion, repoVersion+1)
}

// Shamelessly lifted from FSRepo
func (m *MigrationRunner) loadVersion() (uint, error) {
	// TODO: limited file reading, to avoid attack vector
	file, err := ioutil.ReadFile(filepath.Join(m.oldRepoOpt, "version"))
	if err != nil {
		return 0, err
	}

	version, err := strconv.Atoi(strings.Trim(string(file), "\n"))
	if err != nil {
		return 0, errors.New("corrupt version file: version is not an integer")
	}

	return uint(version), nil
}
