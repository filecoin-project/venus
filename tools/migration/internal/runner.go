package internal

import (
	"fmt"
	"io/ioutil"
	"log"
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

	// Versions returns valid from and to migration versions for this migration.
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

// MigrationRunner represents a migration command
type MigrationRunner struct {
	// logger logs to stdout/err and a logfile.
	logger     *Logger

	// command is the migration command to run, passed from the CLI
	command string

	// oldRepoOpt is the option passed  from the CLI.
	oldRepoOpt string

	// MigrationsProvider is a dependency for fetching available migrations
	// to allow unit tests to supply test migrations without creating test fixtures.
	MigrationsProvider func() []Migration
}

// NewMigrationRunner builds a MigrationRunner for the given command and repo options
func NewMigrationRunner(logger *Logger, command, oldRepoOpt string) *MigrationRunner {
	return &MigrationRunner{
		logger:     logger,
		command:    command,
		oldRepoOpt: oldRepoOpt,
		MigrationsProvider: DefaultMigrationsProvider,
	}
}

// Run executes the MigrationRunner
func (m *MigrationRunner) Run() error {
	repoVersion, err := m.GetSourceRepoVersion()
	if err != nil {
		return err
	}
	if repoVersion == m.getTargetMigrationVersion() {
		return fmt.Errorf("binary version %d = repo version %d; migration not run", repoVersion, m.getTargetMigrationVersion())
	}

	for _, mig := range m.MigrationsProvider() {
		from, to := mig.Versions()
		if to != from+1 {
			log.Printf("Refusing multi-version migration from %d to %d", from, to)
			continue
		}
		if from == repoVersion {
			newRepoPath, err := CloneRepo(m.oldRepoOpt)
			if err != nil {
				return err
			}
			return m.runCommand(mig, to, newRepoPath)
		}
	}
	// else exit with error
	m.logger.Error(fmt.Errorf("did not find valid repo migration for version %d to version %d", repoVersion, repoVersion+1))
	return m.logger.Close()
}

func (m *MigrationRunner) runCommand(mig Migration, to uint, newRepoPath string) error {
	var err error
			switch m.command {
			case "migrate":
				if err = mig.Migrate(newRepoPath); err != nil {
					return err
				}
				if err = m.validateAndUpdateVersion(to, newRepoPath, mig); err != nil {
					return err
				}
				if err = InstallNewRepo(m.oldRepoOpt, newRepoPath); err != nil {
					return err
				}
			case "buildonly":
				if err = mig.Migrate(newRepoPath); err != nil {
					return err
				}
			case "install":
				if err = m.validateAndUpdateVersion(to, newRepoPath, mig); err != nil {
					return err
				}
				if err = InstallNewRepo(m.oldRepoOpt, newRepoPath); err != nil {
					return err
				}
			}
			return m.logger.Close()
		}
	}
}

// Shamelessly lifted from FSRepo, with version checking added.
func (m *MigrationRunner) GetSourceRepoVersion() (uint, error) {
	file, err := ioutil.ReadFile(filepath.Join(m.oldRepoOpt, repo.VersionFilename()))
	if err != nil {
		return 0, err
	}

	strVersion := strings.Trim(string(file), "\n")
	version, err := strconv.Atoi(strVersion)
	if err != nil {
		return 0, err
	}

	if version < 0 || version > 10000 {
		return 0, fmt.Errorf("repo version out of range: %s", strVersion)
	}

	return uint(version), nil
}

func (m *MigrationRunner) validateAndUpdateVersion(toVersion uint, newRepoPath string, mig Migration) error {
	if err := mig.Validate(m.oldRepoOpt, newRepoPath); err != nil {
		return err
	}
	toVersionStr := fmt.Sprintf("%d", toVersion)
	if err := ioutil.WriteFile(filepath.Join(newRepoPath, "version"), []byte(toVersionStr), 0644); err != nil {
		return err
	}
	return nil
}

func (m *MigrationRunner) getTargetMigrationVersion() uint {
	targetVersion := uint(0)
	migrations := m.MigrationsProvider()
	for _, mig := range migrations {
		_, to := mig.Versions()
		if to > targetVersion {
			targetVersion = to
		}
	}
	return targetVersion
}
