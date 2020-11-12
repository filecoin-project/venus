package commands

import (
	"github.com/filecoin-project/go-state-types/abi"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var drandCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Explore access and configure drand.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"random": drandRandom,
	},
}

var drandRandom = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Retrieve randomness round from drand group",
	},
	Options: []cmds.Option{
		cmds.Uint64Option("round", "retrieve randomness at given round (default 0)"),
		cmds.Uint64Option("round", "retrieve randomness at height round (default 0)"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		round, _ := req.Options["round"].(uint64)
		height, _ := req.Options["height"].(uint64)

		entry, err := GetDrandAPI(env).GetEntry(req.Context, abi.ChainEpoch(height), round)
		if err != nil {
			return err
		}
		return re.Emit(entry)
	},
}
