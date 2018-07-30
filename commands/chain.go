// Package commands implements the command to print the blockchain.
package commands

import (
	"gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ts := GetNode(env).ChainMgr.GetHeaviestTipSet()
		if len(ts) == 0 {
			return ErrHeaviestTipSetNotFound
		}
		var out []*cid.Cid
		for it := ts.ToSortedCidSet().Iter(); !it.Complete(); it.Next() {
			out = append(out, it.Value())
		}
		re.Emit(out) //nolint: errcheck
		return nil
	},
	Type: []*cid.Cid{},
}

var chainLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "dump full block chain",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		for raw := range GetNode(env).ChainMgr.BlockHistory(req.Context) {
			switch v := raw.(type) {
			case error:
				return v
			case core.TipSet:
				if len(v) == 0 {
					panic("tipsets from this channel should have at least one member")
				}
				re.Emit(v.ToSlice()) // nolint: errcheck
			default:
				return errors.New("unexpected type")
			}
		}

		return nil
	},
	Type: []types.Block{},
}
