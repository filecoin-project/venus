package repo01

type Migration struct{}

func (m *Migration) Describe() string {
	return "Update version file from 0 to 1"
}

func (m *Migration) Migrate(newRepoPath string) error {
	// Update version file from 0 to 1
	// The runner takes care of this itself so this is a noop
	return nil
}

func (m *Migration) Versions() (uint, uint) {
	return 0, 1
}

func (m *Migration) Validate(oldRepoPath, newRepoPath string) error {
	// Nothing to validate as version bump happens afer validation in runner
	return nil
}

