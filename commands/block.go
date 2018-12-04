package commands

import (
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var showCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get human-readable representations of filecoin objects",
	},
	Subcommands: map[string]*cmds.Command{
		"block": showBlockCmd,
	},
}

var showBlockCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show a Filecoin block by its CID",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ref", true, false, "CID of block to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		block, err := GetAPI(env).Block().Get(req.Context, cid)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if err = cmds.EmitOnce(re, block); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
	},
}
