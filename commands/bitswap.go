package commands

import (
	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
)

var bitswapCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Explore libp2p bitswap.",
	},

	Subcommands: map[string]*cmds.Command{
		"stats": statsBitswapCmd,
	},
}

var statsBitswapCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show bitswap statistics.",
	},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		stats, err := GetPorcelainAPI(env).BitswapGetStats(req.Context)
		if err != nil {
			return err
		}
		return res.Emit(stats)
	},
	Type: &bitswap.Stat{},
}
