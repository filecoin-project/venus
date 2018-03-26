// Package commands implements the command to print the blockchain.
package commands

import (
	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/types"
)

var chainCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Inspect the filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"ls": chainLsCmd,
	},
}

var chainLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "dump full block chain",
	},
	Run: chainRun,
	Encoders: cmds.EncoderMap{
		cmds.JSON: cmds.Encoders[cmds.JSON],
		cmds.Text: cmds.Encoders[cmds.JSON], // TODO: devise a better text representation than JSON (e.g. columnar)
	},
	Type: types.Block{},
}

func chainRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	n := GetNode(env)

	blk := n.ChainMgr.GetBestBlock()
	re.Emit(blk) // nolint: errcheck

	for blk.Parent != nil {
		var next types.Block
		if err := n.CborStore.Get(req.Context, blk.Parent, &next); err != nil {
			return err
		}
		re.Emit(next) // nolint: errcheck
		blk = &next
	}

	return nil
}
