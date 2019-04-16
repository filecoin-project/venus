package cmd

import (
	"fmt"
	"os"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/tools/migration/internal"
	"github.com/filecoin-project/go-filecoin/tools/migration/migrate_1-to-2"
)

type Migrator interface {
	Migrate(oldRepo, newRepo *os.File) error
	Describe()
	Validate(oldRepo, newRepo *os.File) error
}

// Run runs the given migration command
func Run(cmd string, verbose bool) error {
	var err error
	log := logging.Logger("Migration runner")
	logpath := "/tmp/migration_log.txt"

	var oldRepo, newRepo *os.File
	var oldpath string
	filpath := os.Getenv("FIL_PATH")
	if filpath == "" {
		homeDir := os.Getenv("HOME")
		oldpath = fmt.Sprintf("%s/.filecoin", homeDir)
	} else {
		oldpath = filpath
	}
	oldRepo, err = os.Open(oldpath)
	if err != nil {
		log.Error(err)
		return err
	}

	// open repodir read-only
	// create new repodir & open read-write
	newRepo, err = os.Create("/tmp/somedir")
	if err != nil {
		log.Error(err)
		return err
	}

	var migrator Migrator
	miglog, err := makeMigl()
	if err != nil {
		miglog.Error(err)
		return err
	}
	// TODO see Issue #2585
	migrator = migrate_1_to_2.NewMigrator_1_2(miglog)

	switch cmd {
	case "describe":
		migrator.Describe()
	case "buildonly":
		err = migrator.Migrate(oldRepo, newRepo)
	case "install":
		if err = install(oldRepo, newRepo); err != nil {
			miglog.Error(err)
		}
	case "migrate":
		if err = migrator.Migrate(oldRepo, newRepo); err != nil {
			miglog.Error(err)
		}

		if err = migrator.Validate(oldRepo, newRepo); err != nil {
			miglog.Error(err)
		}

		if err = install(oldRepo, newRepo); err != nil {
			miglog.Error(err)
		}
	}
	if err != nil {
		miglog.Error(err)
		return err
	}
	miglog.Print(fmt.Sprintf("Done.  Please see log for more detail: ", logpath))
	return nil
}

// swap old repo for new
func install(oldRepo, newRepo *os.File) error {
	return nil
}

// make a new logger
func makeMigl() (*internal.Migl, error) {
	logfile, err := os.Create("/tmp/foo.txt") // truncates
	if err != nil {
		return nil, err
	}
	migl := internal.NewMigl(logfile, true)
	return &migl, nil
}
