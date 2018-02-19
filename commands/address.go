package commands

import (
	"fmt"
	"io"
	"math/big"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

var walletCmd = &cmds.Command{
	Subcommands: map[string]*cmds.Command{
		"addrs": addrsCmd,
	},
}

var addrsCmd = &cmds.Command{
	Subcommands: map[string]*cmds.Command{
		"balance": addrsBalanceCmd,
		"list":    addrsListCmd,
		"new":     addrsNewCmd,
	},
}

type addressResult struct {
	Address string
}

var addrsNewCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)

		addr := fcn.Wallet.NewAddress()

		re.Emit(&addressResult{addr.String()}) // nolint: errcheck
	},
	Type: addressResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *addressResult) error {
			_, err := fmt.Fprintln(w, a.Address)
			return err
		}),
	},
}

var addrsListCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)
		var out []addressResult
		for _, a := range fcn.Wallet.GetAddresses() {
			out = append(out, addressResult{a.String()})
		}
		re.Emit(out) // nolint: errcheck
	},
	Type: []addressResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, addrs []addressResult) error {
			for _, a := range addrs {
				_, err := fmt.Fprintln(w, a.Address)
				if err != nil {
					return err
				}
			}
			return nil
		}),
	},
}

var addrsBalanceCmd = &cmds.Command{
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, false, "address to get balance for"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)
		blk := fcn.ChainMgr.GetBestBlock()
		if blk.StateRoot == nil {
			re.SetError("state root in latest block was nil", cmdkit.ErrNormal)
			return
		}

		tree, err := types.LoadStateTree(req.Context, fcn.CborStore, blk.StateRoot)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		toAddr, err := types.ParseAddress(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		msg := types.NewMessage(types.Address(""), types.Address("token"), "balance", []interface{}{toAddr})
		receipt, err := core.ApplyMessage(req.Context, tree, msg)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		balance := big.NewInt(0)
		balance.SetBytes(receipt.Return)

		re.Emit(balance) // nolint: errcheck
	},
	Type: big.Int{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, b *big.Int) error {
			_, err := fmt.Fprintln(w, b.String())
			return err
		}),
	},
}
