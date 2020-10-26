// Package commands implements the command to print the blockchain.
package commands

import (
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var dagCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with IPLD DAG objects.",
	},
	Subcommands: map[string]*cmds.Command{
		"get": dagGetCmd,
	},
}

var dagGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a DAG node by its CID",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ref", true, false, "CID of object to get"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		out, err := GetPorcelainAPI(env).DAGGetNode(req.Context, req.Arguments[0])
		if err != nil {
			return err
		}

		return re.Emit(out)
	},
}
