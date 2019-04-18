package internal

import (
	"os"
)

type Migrator interface {
	Migrate(newRepo *os.File) error
	Describe() string
	Validate(oldRepo, newRepo *os.File) error
}

// TODO: Issue #2585 Implement repo migration version detection and upgrade decisioning
type MigrationRunner struct{}

func (m *MigrationRunner) Run(cmd string, verbose bool) error {

	// TODO: Issue #2595 Implement first repo migration which is just a version bump
	return nil
}
