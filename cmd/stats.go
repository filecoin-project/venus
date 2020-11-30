package cmd

import (
	"github.com/filecoin-project/venus/app/node"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-core/metrics"
)

var statsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "View various filecoin node statistics",
	},
	Subcommands: map[string]*cmds.Command{
		"bandwidth": statsBandwidthCmd,
	},
}

var statsBandwidthCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "View bandwidth usage metrics",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		bandwidthStats := env.(*node.Env).NetworkAPI.NetworkGetBandwidthStats()

		return re.Emit(bandwidthStats)
	},
	Type: metrics.Stats{},
}
