package internal

import (
	"os"
)

type Migrator interface {
	Migrate(newRepo *os.File) error
	Describe() string
	Validate(oldRepo, newRepo *os.File) error
}

type MigrationRunner struct{}

func (m *MigrationRunner) Run(cmd string, verbose bool) error {
	return nil
}
