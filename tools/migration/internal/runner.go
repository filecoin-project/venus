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
	Migrate(newRepoPath string) error

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
	Validate(oldRepoPath, newRepoPath string) error
}

// RepoWrangler is a helper that manages filesystem operations and figures out what the correct paths
// are for everything.
type RepoWrangler interface {
	// GetOldRepo opens and returns the old repo with read-only access
	GetOldRepo() (*os.File, error)
	// CloneRepo creates and returns the new repo dir with read/write permissions
	CloneRepo() error
	// GetOldRepoPath returns the full path of the old repo
	GetOldRepoPath() string
	// GetNewRepoPath returns the full path of the old repo
	GetNewRepoPath() string
}

type MigrationRunner struct {
	verbose bool
	command string
	helper  RepoWrangler
}

func NewMigrationRunner(verb bool, command, oldRepoOpt, newRepoPrefixOpt string) *MigrationRunner {
	// TODO: Issue #2585 Implement repo migration version detection and upgrade decisioning

	helper := NewRepoFSWrangler(oldRepoOpt, newRepoPrefixOpt)
	return &MigrationRunner{
		verbose: verb,
		command: command,
		helper:  helper,
	}
}

func (m *MigrationRunner) Run() error {
	// TODO: Issue #2595 Implement first repo migration
	return nil
}
