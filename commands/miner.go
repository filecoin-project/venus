package commands

import (
	"context"
	"fmt"
	"io"
	"math/big"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/core"
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
		fcn.MsgPool.Add(reward)

		tree, err := types.LoadStateTree(req.Context, fcn.CborStore, cur.StateRoot)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		processBlock := func(ctx context.Context, b *types.Block) error {
			return core.ProcessBlock(ctx, b, tree)
		}
		flushTree := func(ctx context.Context) (*cid.Cid, error) {
			return tree.Flush(ctx)
		}
		next, err := mining.BlockGenerator{Mp: fcn.MsgPool}.Generate(req.Context, cur, processBlock, flushTree)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if err := fcn.AddNewBlock(req.Context, next); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(next.Cid()) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			fmt.Fprintln(w, c)
			return nil
		}),
	},
}
