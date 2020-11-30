package cmd

import (
	"github.com/filecoin-project/venus/app/node"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

// BootstrapLsResult is the result of the bootstrap listing command.
type BootstrapLsResult struct {
	Peers []string
}

var bootstrapCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with bootstrap addresses",
	},
	Subcommands: map[string]*cmds.Command{
		"ls": bootstrapLsCmd,
	},
}

var bootstrapLsCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		peers, err := env.(*node.Env).ConfigAPI.ConfigGet("bootstrap.addresses")
		if err != nil {
			return err
		}

		return re.Emit(&BootstrapLsResult{peers.([]string)})
	},
	Type: &BootstrapLsResult{},
}
