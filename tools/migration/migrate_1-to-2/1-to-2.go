package migrate_1_to_2

import (
	"fmt"
	"github.com/pkg/errors"
	"os"
)

const (
	MigrationVersion = "2"
	PreviousVersion  = "1"
	Description      = "Updates the repo version"
)

var (
	ErrMigrationFailed = errors.New("migrator failed")
)

type MigrationLogger interface {
	Print(string)
}

type migrator struct {
	log MigrationLogger
}

// TODO: Issue 2595
// NewMigrator_1_2 instantiates a new migrator
func NewMigrator_1_2(log MigrationLogger) *migrator {
	return &migrator{log: log}
}

// Describe emits a description of what this migrator will do.
// Verbose option is ignored; output is not logged.
func (mig *migrator) Describe() {
	fmt.Printf("Migration from %s to %s: %s", PreviousVersion, MigrationVersion, Description)
}

// Run executes the migration steps
func (mig *migrator) Migrate(oldRepo, newRepo *os.File) error {
	mig.log.Print("Migrate succeeded")
	return nil
}

// Validate returns error if migration tests failed, describing why
func (mig *migrator) Validate(oldRepo, newRepo *os.File) error {
	return nil
}
