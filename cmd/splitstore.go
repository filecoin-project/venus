package cmd

import (
	"fmt"

	"github.com/filecoin-project/venus/app/node"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var splitstoreCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manage splitstore",
	},
	Subcommands: map[string]*cmds.Command{
		"rollback": splitstoreRollbackCmd,
	},
}

var splitstoreRollbackCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Rollback splitstore to badger store",
	},
	PreRun: func(req *cmds.Request, env cmds.Environment) error {
		fmt.Println("It may take a while to transfer block ...")
		return nil
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		return env.(*node.Env).SplitstoreAPI.Rollback()
	},
}
