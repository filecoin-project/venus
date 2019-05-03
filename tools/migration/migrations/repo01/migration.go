package repo01

// Migration is the repo migration transforming repo version 0 to repo version 1.
type Migration struct{}

// Describe outputs description of migration 0 ==> 1
func (m *Migration) Describe() string {
	return "Update version file from 0 to 1"
}

// Migrate performs migration 0 ==> 1
func (m *Migration) Migrate(newRepoPath string) error {
	// Update version file from 0 to 1
	// The runner takes care of this itself so this is a noop
	return nil
}

// Versions outputs the to and from versions for migration 0 ==> 1
func (m *Migration) Versions() (uint, uint) {
	return 0, 1
}

// Validate does validation of migration 0 ==> 1
func (m *Migration) Validate(oldRepoPath, newRepoPath string) error {
	// Nothing to validate as version bump happens afer validation in runner
	return nil
}
