package commands

import (
	"fmt"
	"io"
	"math/big"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/mining"
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

		cur := fcn.ChainMgr.GetBestBlock()

		addrs := fcn.Wallet.GetAddresses()
		if len(addrs) == 0 {
			re.SetError("no addresses in wallet to mine to", cmdkit.ErrNormal)
			return
		}
		myaddr := addrs[0]

		reward := types.NewMessage(types.Address("filecoin"), myaddr, big.NewInt(1000), "", nil)
		if err := fcn.MsgPool.Add(reward); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		tree, err := types.LoadStateTree(req.Context, fcn.CborStore, cur.StateRoot)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		w, err := mining.NewWorker(cur, mining.NewBlockGenerator(fcn.MsgPool), tree)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
		}
		res := <-w.Start(req.Context)
		if res.Err != nil {
			re.SetError(res.Err, cmdkit.ErrNormal)
			return
		}
		fcn.AddNewBlock(req.Context, res.NewBlock)
		re.Emit(res.NewBlock.Cid())
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			fmt.Fprintln(w, c)
			return nil
		}),
	},
}
