package commands

import (
	"fmt"
	"io"

	cmds "gx/ipfs/QmWGgKRz5S24SqaAapF5PPCfYfLT7MexJZewN5M82CQTzs/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/types"
)

var minerCmd = &cmds.Command{
	Subcommands: map[string]*cmds.Command{
		"gen-block": minerGenBlockCmd,
	},
}

var minerGenBlockCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)

		cur := fcn.ChainMgr.BestBlock

		next := &types.Block{
			Parent: cur.Cid(),
			Height: cur.Height + 1,
		}

		if err := fcn.ChainMgr.ProcessNewBlock(req.Context, next); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(next.Cid())
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			fmt.Fprintln(w, c)
			return nil
		}),
	},
}
