// Package commands implements the command to print the blockchain.
package commands

import (
	"encoding/json"
	"io"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/types"
)

var chainCmd = &cmds.Command{
	Subcommands: map[string]*cmds.Command{
		"ls": chainLsCmd,
	},
}

var chainLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "dump full block chain",
	},
	Run:  chainRun,
	Type: types.Block{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(chainTextEncoder),
	},
}

func chainRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
	n := GetNode(env)

	blk := n.ChainMgr.GetBestBlock()
	re.Emit(blk)

	for blk.Parent != nil {
		var next types.Block
		if err := n.CborStore.Get(req.Context, blk.Parent, &next); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(&next)
		blk = &next
	}
}

func chainTextEncoder(req *cmds.Request, w io.Writer, val *types.Block) error {
	marshaled, err := json.MarshalIndent(val, "", "\t")
	if err != nil {
		return err
	}
	marshaled = append(marshaled, byte('\n'))
	_, err = w.Write(marshaled)
	return err
}
