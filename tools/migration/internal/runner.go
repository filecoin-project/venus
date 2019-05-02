package internal

import (
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

	Versions() (from string, to string)

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
	logger     *Logger
	command    string
	oldRepoOpt string
	MigrationsProvider func() []Migration
}

// NewMigrationRunner builds a MirgrationRunner for the given command and repo options
func NewMigrationRunner(logger *Logger, command, oldRepoOpt string) *MigrationRunner {
	// TODO: Issue #2585 Implement repo migration version detection and upgrade decisioning

	return &MigrationRunner{
		logger:     logger,
		command:    command,
		oldRepoOpt: oldRepoOpt,
	}
}

// Run executes the MigrationRunner
func (m *MigrationRunner) Run() error {
	// TODO: Issue #2595 Implement first repo migration
	// detect current version of old repo
	_, err := repo.OpenFSRepo(m.oldRepoOpt)
	if err != nil {
		return err
	}

	// iterate over migrations provided by the Provider.
		// look for a migration that upgrades the old repo by 1 version
		// if there is one,
			// perform the migration command
	// else exit with error
	return m.logger.Close()
}
