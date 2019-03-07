package commands

import (
	"gx/ipfs/QmQtQrtNioesAWtrx8csBvfY37gTe94d6wQ3VikZUjxD39/go-ipfs-cmds"
	"gx/ipfs/QmZZseAa9xcK6tT3YpaShNUAEpyRAoWmUL5ojH3uGNepAc/go-libp2p-metrics"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
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
