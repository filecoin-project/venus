// Package commands implements the command to print the blockchain.
package commands

import (
	"gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

var chainCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Inspect the filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"head": chainHeadCmd,
		"ls":   chainLsCmd,
	},
}

var chainHeadCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "get heaviest tipset CIDs",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		ts := GetNode(env).ChainMgr.GetHeaviestTipSet()
		if len(ts) == 0 {
			re.SetError(ErrHeaviestTipSetNotFound, cmdkit.ErrNormal)
			return
		}
		var out []*cid.Cid
		for it := ts.ToSortedCidSet().Iter(); !it.Complete(); it.Next() {
			out = append(out, it.Value())
		}
		re.Emit(out) //nolint: errcheck
	},
	Type: []*cid.Cid{},
}

var chainLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "dump full block chain",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		for raw := range GetNode(env).ChainMgr.BlockHistory(req.Context) {
			switch v := raw.(type) {
			case error:
				re.SetError(v, cmdkit.ErrNormal)
				return
			case core.TipSet:
				if len(v) == 0 {
					panic("tipsets from this channel should have at least one member")
				}
				re.Emit(v.ToSlice()) // nolint: errcheck
			default:
				re.SetError("unexpected type", cmdkit.ErrNormal)
				return
			}
		}
	},
	Type: []types.Block{},
}
