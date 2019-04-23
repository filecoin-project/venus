package internal

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

// MigrationRunner represent a migration command
type MigrationRunner struct {
	verbose    bool
	command    string
	oldRepoOpt string
}

// NewMigrationRunner builds a MirgrationRunner for the given command and repo options
func NewMigrationRunner(verb bool, command, oldRepoOpt string) *MigrationRunner {
	// TODO: Issue #2585 Implement repo migration version detection and upgrade decisioning

	return &MigrationRunner{
		verbose:    verb,
		command:    command,
		oldRepoOpt: oldRepoOpt,
	}
}

// Run executes the MigrationRunner
func (m *MigrationRunner) Run() error {
	// TODO: Issue #2595 Implement first repo migration
	return nil
}
