package migrate_1_to_2

import (
	"os"

	"github.com/pkg/errors"
)

const (
	Description      = "a test migrator that just updates the repo version"
	MigrationVersion = "0.2"
	PreviousVersion  = "0.1"
)

var (
	ErrMigrationFailed = errors.New("migrator failed")
)

//  Migration runner defines an interface which migrator code must satisfy.
//  Migrations are a pure function, given access to the input (read-only) and
//  output repos, or a read-write repo to be migrated in place.

type MigrationLogger interface {
	Debug(string)
	Error(string)
	Info(string)
	Warn(string)
}

type migrator struct {
	log MigrationLogger
}

// NewMigrator instantiates a new migrator
func NewMigrator_1_2(log MigrationLogger) *migrator {
	return &migrator{log: log}
}

// Describe emits a description of what this migrator will do.
// Verbose option is ignored; output is not logged.
func (mig *migrator) Describe() {
	mig.log.Info(Description)
	// use the emitter to output description
}

// Run runs the migrator steps on a copy of the repo
func (mig *migrator) Migrate(oldRepo, newRepo *os.File) error {
	// copyData()
	// migrateStep1
	// migrateStep2
	// migrateStep3
	mig.log.Info("Migrate succeeded")
	return nil
}

// Validate returns error if migration tests failed, describing why
func (mig *migrator) Validate(oldRepo, newRepo *os.File) error {
	return nil
}
