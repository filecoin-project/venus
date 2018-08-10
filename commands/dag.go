// Package commands implements the command to print the blockchain.
package commands

import (
	cmds "gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		out, err := GetAPI(env).Dag().Get(req.Context, req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(out) // nolint: errcheck
	},
}
