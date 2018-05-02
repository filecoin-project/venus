package commands

import (
	"gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

var showCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get human-readable representations of Filecoin objects",
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return errors.New("could not parse argument as CID")
		}

		block, err := GetNode(env).ChainMgr.FetchBlock(req.Context, cid)
		if err != nil {
			return err
		}

		if err = cmds.EmitOnce(re, block); err != nil {
			return err
		}

		return nil
	},
}
