package cmd

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/venus/app/node"
)

var drandCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Explore drand",
	},
	Subcommands: map[string]*cmds.Command{
		"random": drandRandomCmd,
	},
}

var drandRandomCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Retrieve randomness from the drand server",
	},
	Options: []cmds.Option{
		cmds.Uint64Option("height", "chain epoch (default 0)"),
		cmds.Uint64Option("round", "retrieve randomness at requested round (default 0)"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		round, _ := req.Options["round"].(uint64)
		height, _ := req.Options["height"].(uint64)

		entry, err := env.(*node.Env).ChainAPI.GetEntry(req.Context, abi.ChainEpoch(height), round)
		if err != nil {
			return err
		}
		return re.Emit(entry)
	},
}
