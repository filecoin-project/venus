package migration

import (
	"fmt"

	"github.com/filecoin-project/go-filecoin/config"

	"github.com/filecoin-project/go-filecoin/tools/migration/migrate_1-to-2"
	"os"

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/repo"
)

// runner

type Migrator interface {
	Migrate(oldRepo, newRepo *os.File) error
	Describe()
	Validate(oldRepo, newRepo *os.File) error
}

func Run(req *cmds.Request) {
	var err error
	log := logging.Logger("Migration runner")
	// TODO: proper log path
	logpath := "/tmp/migration_log.txt"
	// set up logfile & writing to logfile

	// 1. do version checking
	// look at this version
	// look at repo version
	// if this version is too high, exit with error

	// look at the options
	// repodir is:
	// 1) FIL_PATH 2) default
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
		return // or return error, something.
	}

	// open repodir read-only
	// create new repodir & open read-write
	newRepo, err = os.Create("/tmp/somedir")
	if err != nil {
		log.Error("failed to create dir: ")
		log.Error(err)
		return
	}

	var mig Migrator
	// figure out which migrator to run and run it ?
	// or do we figure out what migration + runner to build and build it, and then run?
	// or does this line get updated to the latest migrator?
	// or ?
	mgl, err := makeMigl()
	if err != nil {
		log.Fatalf("could not make Migl logger, %v", err)
	}
	mig = migrate_1_to_2.NewMigrator_1_2(mgl)

	switch req.Arguments[0] {
	case "Describe":
		mig.Describe()
		err = nil
	case "BuildOnly":
		err = mig.Migrate(oldRepo, newRepo)
	case "Install":
		// runner controls the installation, not migrator
		// check that the migration completed successfully
		if err = install(oldRepo, newRepo); err != nil {
			log.Error(err)
			return
		}
	case "Run":
		if err = mig.Migrate(oldRepo, newRepo); err != nil {
			log.Error(err) // replace these with a call to a logging/output func that respects verbose
			return
		}

		if err = mig.Validate(oldRepo, newRepo); err != nil {
			log.Error(err)
			return
		}
		// run install
		if err = install(oldRepo, newRepo); err != nil {
			log.Error(err)
			return
		}
	}
	if err != nil {
		// don't panic
		log.Errorf(err.Error())
	}
	log.Info("============================================================================================")
	log.Infof("Done.  Please see log for more detail: %s", logpath)
	log.Info("============================================================================================")
}

// fs access and we should know the location since we know the previous version.
func getOldFSRepo(req *cmds.Request) (repo.FSRepo, error) {
	return repo.FSRepo{}, nil
}

// fs access in the old repo
func getCurrentRepoVersion(req *cmds.Request) string {
	return ""
}

// 	getNewRepoVersion() string
// also just fs access
func getOldConfig() *config.Config {
	return &config.Config{}
}

// swap old repo for new
func install(oldRepo, newRepo *os.File) error {
	return nil
}

func makeMigl() (*Migl, error) {
	logfile, err := os.OpenFile("/tmp/foo.txt", os.O_WRONLY, os.ModeExclusive)
	if err != nil {
		return nil, err
	}
	migl := NewMigl(logfile)
	return &migl, nil
}
