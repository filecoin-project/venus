package commands

import (
	"fmt"
	"io"
	"math/big"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
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
		fmt.Println("Building on block: ", cur.Height)

		myaddr := fcn.Wallet.GetAddresses()[0]

		reward := &types.Message{
			From:  types.Address("filecoin"),
			To:    myaddr,
			Value: big.NewInt(1000),
		}

		msgs := []*types.Message{reward}
		msgs = append(msgs, fcn.MsgPool.Pending()...)

		next := &types.Block{
			Parent:   cur.Cid(),
			Height:   cur.Height + 1,
			Messages: msgs,
		}

		tree, err := state.LoadTree(req.Context, fcn.CborStore, cur.StateRoot)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if err := core.ProcessBlock(req.Context, next, tree); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		stcid, err := tree.Flush(req.Context)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		next.StateRoot = stcid

		if err := fcn.AddNewBlock(req.Context, next); err != nil {
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
