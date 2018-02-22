package commands

import (
	"fmt"
	"io"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
)

var minerCmd = &cmds.Command{
	Subcommands: map[string]*cmds.Command{
		"gen-block": minerGenBlockCmd,
	},
}

var minerGenBlockCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)

		cur := fcn.ChainMgr.GetBestBlock()

		addrs := fcn.Wallet.GetAddresses()
		if len(addrs) == 0 {
			re.SetError("no addresses in wallet to mine to", cmdkit.ErrNormal)
			return
		}
		myaddr := addrs[0]

		worker := mining.NewWorker(
			mining.NewBlockGenerator(fcn.MsgPool, fcn.CborStore, core.ProcessBlock, myaddr),
			fcn.AddNewBlock,
		)
		nextCid, err := worker.Mine(req.Context, cur)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		re.Emit(nextCid)
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			fmt.Fprintln(w, c)
			return nil
		}),
	},
}
