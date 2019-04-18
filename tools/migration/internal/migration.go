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

// TODO: Issue #2585 Implement repo migration version detection and upgrade decisioning
type MigrationRunner struct{}

func (m *MigrationRunner) Run(cmd string, verbose bool) error {

	// TODO: Issue #2595 Implement first repo migration which is just a version bump
	return nil
}
