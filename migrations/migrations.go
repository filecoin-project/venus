package migrations

import (
	"fmt"
	"os"

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log"
)

// runner

type Migrator interface {
	Migrate() error
	Describe()
	BuildOnly() error
	Install() error
	Validate() error
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
	mig = NewMigrator_1_2(oldRepo, newRepo, true)

	// TODO: do these calls with reflections?
	switch req.Arguments[0] {
	case "Describe":
		mig.Describe()
		err = nil
	case "BuildOnly":
		err = mig.BuildOnly()
	case "Install":
		// runner controls the installation, not migrator:
		err = mig.Install()
	case "Run":
		if err = mig.Migrate(); err != nil {
			log.Error(err)
		} else {
			err = mig.Install()
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
