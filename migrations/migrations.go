package migrations

import (
	"os"
	cmds "github.com/ipfs/go-ipfs-cmds"
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
	// 1. do version checking
		// look at this version
		// look at repo version
		// if this version is too high, exit with error

	// look at the options
	// repodir is:
	// 1) FIL_PATH 2) default
	// open repodir read-only
	// make new repodir read-write
	// instantiate migrator with new repo, old repo, verbose
	// switch on command & call appropriate  dir
	// runner controls the installation, not migrator
}