package internal

import (
	"errors"
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
	logger *Logger

	// command is the migration command to run, passed from the CLI
	command string

	// oldRepoOpt is the value of --old-repo passed  from the CLI
	oldRepoOpt string

	// newRepoOpt is value of --new-repo passed from the CLI
	// required for 'install' command
	// blank for 'describe', 'buildonly' and 'migrate' commands
	newRepoOpt string

	// MigrationsProvider is a dependency for fetching available migrations
	// to allow unit tests to supply test migrations without creating test fixtures.
	MigrationsProvider func() []Migration
}

// NewMigrationRunner builds a MigrationRunner for the given command and repo options
func NewMigrationRunner(logger *Logger, command, oldRepoOpt, newRepoOpt string) *MigrationRunner {
	return &MigrationRunner{
		logger:             logger,
		command:            command,
		oldRepoOpt:         oldRepoOpt,
		newRepoOpt:         newRepoOpt,
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

	var mig Migration
	if mig, err = m.getValidMigration(repoVersion); err != nil {
		return fmt.Errorf("migration check failed: %s", err.Error())
	}
	return m.runCommand(mig)
}

func (m *MigrationRunner) runCommand(mig Migration) error {
	var err error

	_, to := mig.Versions()

	switch m.command {
	case "describe":
		m.logger.Print(mig.Describe())
	case "migrate":
		newRepoPath, err := CloneRepo(m.oldRepoOpt)
		if err != nil {
			return err
		}
		if err = mig.Migrate(newRepoPath); err != nil {
			return errors.New("migration failed: " + err.Error())
		}
		if err = m.validateAndUpdateVersion(to, newRepoPath, mig); err != nil {
			return errors.New("validation failed: " + err.Error())
		}
		if err = InstallNewRepo(m.oldRepoOpt, newRepoPath); err != nil {
			return errors.New("installation failed: " + err.Error())
		}
	case "buildonly":
		newRepoPath, err := CloneRepo(m.oldRepoOpt)
		if err != nil {
			return err
		}
		if err = mig.Migrate(newRepoPath); err != nil {
			return errors.New("migration failed: " + err.Error())
		}
	case "install":
		if err = m.validateAndUpdateVersion(to, m.newRepoOpt, mig); err != nil {
			return errors.New("validation failed: " + err.Error())
		}
		if err = InstallNewRepo(m.oldRepoOpt, m.newRepoOpt); err != nil {
			return errors.New("installation failed: " + err.Error())
		}
	}
	return nil
}

// GetSourceRepoVersion opens the repo version file and gets the version,
// with version checking added.
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

func (m *MigrationRunner) getValidMigration(repoVersion uint) (mig Migration, err error) {
	var applicableMigs []Migration
	for _, mig := range m.MigrationsProvider() {
		from, to := mig.Versions()
		if to != from+1 {
			log.Printf("Refusing multi-version migration from %d to %d", from, to)
			continue
		}
		if from == repoVersion {
			applicableMigs = append(applicableMigs, mig)
		}
	}
	if len(applicableMigs) > 1 {
		return nil, errors.New("found >1 available migration; cannot proceed")
	}
	if len(applicableMigs) == 0 {
		return nil, fmt.Errorf("did not find valid repo migration for version %d", repoVersion)
	}
	return applicableMigs[0], nil
}
