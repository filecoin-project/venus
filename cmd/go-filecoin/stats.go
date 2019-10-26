package commands

import (
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-core/metrics"
)

var statsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "View various filecoin node statistics",
	},
	Subcommands: map[string]*cmds.Command{
		"bandwidth": statsBandwidthCmd,
	},
}

var statsBandwidthCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "View bandwidth usage metrics",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		bandwidthStats := GetPorcelainAPI(env).NetworkGetBandwidthStats()

		return re.Emit(bandwidthStats)
	},
	Type: metrics.Stats{},
}
