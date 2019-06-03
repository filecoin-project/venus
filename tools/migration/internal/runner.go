package internal

import (
	"fmt"
	"strconv"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"

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

	// oldRepoOpt is the value of --old-repo passed  from the CLI,
	// expanded homedir where needed
	oldRepoOpt string

	// newRepoPath is where the to-be-migrated/migrated repo is located,
	// expanded homedir where needed
	newRepoPath string

	// MigrationsProvider is a dependency for fetching available migrations
	// to allow unit tests to supply test migrations without creating test fixtures.
	MigrationsProvider func() []Migration
}

// NewMigrationRunner builds a MigrationRunner for the given command and repo options
// Returns an error if homepath expansion fails for oldRepoOpt or newRepoOpt
func NewMigrationRunner(logger *Logger, command, oldRepoOpt, newRepoOpt string) (*MigrationRunner, error) {
	oldPath, err := homedir.Expand(oldRepoOpt)
	if err != nil {
		return nil, err
	}
	newPath, err := homedir.Expand(newRepoOpt)
	if err != nil {
		return nil, err
	}
	return &MigrationRunner{
		logger:             logger,
		command:            command,
		oldRepoOpt:         oldPath,
		newRepoPath:        newPath,
		MigrationsProvider: DefaultMigrationsProvider,
	}, nil
}

// RunResult stores the needed results of calling Run()
type RunResult struct {
	// full path to new repo (migrated or not).
	// This is blank unless repo was cloned -- if it errors out too early or for "describe"
	NewRepoPath string

	// Old version and new version. If no applicable migration is found, these will be equal,
	// and if errors early they will = 0.
	OldVersion, NewVersion uint

	// Any errors encountered by Run
	Err error
}

// Run executes the MigrationRunner
func (m *MigrationRunner) Run() RunResult {
	repoVersion, err := m.repoVersion(m.oldRepoOpt)
	if err != nil {
		return RunResult{Err: err}
	}

	targetVersion := m.getTargetMigrationVersion()
	if repoVersion == targetVersion {
		m.logger.Printf("Repo up-to-date: binary version %d = repo version %d", repoVersion, m.getTargetMigrationVersion())
		return RunResult{OldVersion: repoVersion, NewVersion: repoVersion}
	}

	var mig Migration
	if mig, err = m.findMigration(repoVersion); err != nil {
		return RunResult{
			Err:        errors.Wrapf(err, "migration check failed"),
			OldVersion: repoVersion,
			NewVersion: repoVersion,
		}
	}
	// We just didn't find a migration that applies. This is fine.
	if mig == nil {
		return RunResult{
			OldVersion: repoVersion,
			NewVersion: repoVersion,
		}
	}
	err = m.runCommand(mig)
	return RunResult{
		Err:         err,
		OldVersion:  repoVersion,
		NewVersion:  targetVersion,
		NewRepoPath: m.newRepoPath,
	}
}

// runCommand runs the migration command set in the Migration runner.
func (m *MigrationRunner) runCommand(mig Migration) error {
	var err error

	from, to := mig.Versions()

	switch m.command {
	case "describe":
		// Describe is not expected to be run by a script, but by a human, so
		// ignore the logger & print to stdout.
		fmt.Printf("\n   Migration from %d to %d:", from, to)
		fmt.Println(mig.Describe())
	case "migrate":
		if m.newRepoPath, err = CloneRepo(m.oldRepoOpt, to); err != nil {
			return errors.Wrap(err, "clone repo failed")
		}
		m.logger.Printf("new repo will be at %s", m.newRepoPath)

		if err = mig.Migrate(m.newRepoPath); err != nil {
			return errors.Wrap(err, "migration failed")
		}
		if err = m.validateAndUpdateVersion(to, m.newRepoPath, mig); err != nil {
			return errors.Wrap(err, "validation failed")
		}
		if err = InstallNewRepo(m.oldRepoOpt, m.newRepoPath); err != nil {
			return errors.Wrap(err, "installation failed")
		}
	case "buildonly":
		if m.newRepoPath, err = CloneRepo(m.oldRepoOpt, to); err != nil {
			return err
		}
		m.logger.Printf("new repo will be at %s", m.newRepoPath)

		if err = mig.Migrate(m.newRepoPath); err != nil {
			return errors.Wrap(err, "migration failed")
		}
		if err = m.validateAndUpdateVersion(to, m.newRepoPath, mig); err != nil {
			return errors.Wrap(err, "validation failed")
		}
	case "install":
		if m.newRepoPath == "" {
			return errors.New("installation failed: new repo path not specified")
		}

		repoVer, err := repo.ReadVersion(m.newRepoPath)
		if err != nil {
			return errors.Wrap(err, "installation failed")
		}
		// quick check of version to help prevent install after a failed migration
		newVer := strconv.FormatUint(uint64(to), 10) // to forestall parsing errors of version
		if repoVer != newVer {
			return fmt.Errorf("installation failed: repo has version %s, expected version %s", repoVer, newVer)
		}

		if err = InstallNewRepo(m.oldRepoOpt, m.newRepoPath); err != nil {
			return errors.Wrap(err, "installation failed")
		}
	}
	return nil
}

// repoVersion opens the version file for the given version,
// gets the version and validates it
func (m *MigrationRunner) repoVersion(repoPath string) (uint, error) {
	strVersion, err := repo.ReadVersion(repoPath)
	if err != nil {
		return 0, err
	}

	version, err := strconv.Atoi(strVersion)
	if err != nil {
		return 0, errors.Wrap(err, "repo version is corrupt")
	}

	if version < 0 || version > 10000 {
		return 0, errors.Errorf("repo version out of range: %s", strVersion)
	}

	return uint(version), nil
}

// validateAndUpdateVersion calls the migration's validate function and then bumps
// the version number in the new repo.
func (m *MigrationRunner) validateAndUpdateVersion(toVersion uint, newRepoPath string, mig Migration) error {
	if err := mig.Validate(m.oldRepoOpt, newRepoPath); err != nil {
		return err
	}
	return repo.WriteVersion(newRepoPath, toVersion)
}

// getTargetMigrationVersion returns the maximum resulting version of any migration available from the provider.
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

// findMigration finds the list of migrations in the MigrationsProvder that is valid for
//       upgrading current repoVersion
// returns:
//       nil + error if >1 valid migration is found, or
//       the migration to run + nil error
func (m *MigrationRunner) findMigration(repoVersion uint) (mig Migration, err error) {
	var applicableMigs []Migration
	for _, mig := range m.MigrationsProvider() {
		from, to := mig.Versions()
		if to != from+1 {
			m.logger.Printf("refusing multi-version migration from %d to %d", from, to)
		} else if from == repoVersion {
			applicableMigs = append(applicableMigs, mig)
		} else {
			m.logger.Printf("skipping migration from %d to %d", from, to)
		}
	}
	if len(applicableMigs) == 0 {
		m.logger.Printf("did not find valid repo migration for version %d", repoVersion)
		return nil, nil
	}
	if len(applicableMigs) > 1 {
		return nil, fmt.Errorf("found >1 migration for version %d; cannot proceed", repoVersion)
	}
	return applicableMigs[0], nil
}
