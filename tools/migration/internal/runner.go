package internal

import (
	"os"
)

// Migration is the interface to all repo migration versions.
type Migration interface {
	// Migrate performs all migration steps for the Migration that implements the interface.
	// Migrate  expects newRepo to be:
	//		a directory
	// 		read/writeable by this process,
	//      contain a copy of the old repo.
	Migrate(newRepo *os.File) error

	// Describe returns a list of steps, as a formatted string, that a given Migration will take.
	// These should correspond to named functions in the given Migration.
	Describe() string

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
	Validate(oldRepo, newRepo *os.File) error
}

type RepoHelper interface {
	GetOldRepo() (*os.File, error)
	MakeNewRepo() (*os.File, error)
}

// TODO: Issue #2585 Implement repo migration version detection and upgrade decisioning
type MigrationRunner struct {
	verbose bool
	command string
	helper  RepoHelper
}

func NewMigrationRunner(verb bool, command string, rmh RepoHelper) *MigrationRunner {
	return &MigrationRunner{
		verbose: verb,
		command: command,
		helper:  rmh,
	}
}

func (m *MigrationRunner) Run() error {
	// TODO: Issue #2595 Implement first repo migration
	return nil
}
