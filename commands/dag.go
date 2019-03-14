// Package commands implements the command to print the blockchain.
package commands

import (
	cmds "gx/ipfs/QmQtQrtNioesAWtrx8csBvfY37gTe94d6wQ3VikZUjxD39/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var dagCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with IPLD DAG objects.",
	},
	Subcommands: map[string]*cmds.Command{
		"get": dagGetCmd,
	},
}

var dagGetCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get a DAG node by its CID",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ref", true, false, "CID of object to get"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		out, err := GetPorcelainAPI(env).DAGGet(req.Context, req.Arguments[0])
		if err != nil {
			return err
		}

		return re.Emit(out)
	},
}
