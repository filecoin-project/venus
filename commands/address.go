package commands

import (
	"fmt"
	"io"

	"gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

var walletCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage your filecoin wallets",
	},
	Subcommands: map[string]*cmds.Command{
		"addrs":   addrsCmd,
		"balance": balanceCmd,
	},
}

var addrsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with addresses",
	},
	Subcommands: map[string]*cmds.Command{
		"ls":     addrsLsCmd,
		"new":    addrsNewCmd,
		"lookup": addrsLookupCmd,
	},
}

type addressResult struct {
	Address string
}

var addrsNewCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fcn := GetNode(env)
		addr, err := fcn.NewAddress()
		if err != nil {
			return err
		}
		re.Emit(&addressResult{addr.String()}) // nolint: errcheck
		return nil
	},
	Type: &addressResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *addressResult) error {
			_, err := fmt.Fprintln(w, a.Address)
			return err
		}),
	},
}

var addrsLsCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fcn := GetNode(env)
		for _, a := range fcn.Wallet.Addresses() {
			re.Emit(&addressResult{a.String()}) // nolint: errcheck
		}
		return nil
	},
	Type: &addressResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, addr *addressResult) error {
			_, err := fmt.Fprintln(w, addr.Address)
			return err
		}),
	},
}

var addrsLookupCmd = &cmds.Command{
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, false, "miner address to find peerId for"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fcn := GetNode(env)

		address, err := types.NewAddressFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		v, err := fcn.Lookup.GetPeerIDByMinerAddress(req.Context, address)
		if err != nil {
			return errors.Wrapf(err, "failed to find miner with address %s", address.String())
		}
		re.Emit(v.Pretty()) // nolint: errcheck
		return nil
	},
	Type: string(""),
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, pid string) error {
			_, err := fmt.Fprintln(w, pid)
			return err
		}),
	},
}
var balanceCmd = &cmds.Command{
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, false, "address to get balance for"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fcn := GetNode(env)
		// TODO fix #543: Improve UX for multiblock tipset
		blk := fcn.ChainMgr.GetBestBlock()
		if blk.StateRoot == nil {
			return ErrLatestBlockStateRootNil
		}

		tree, err := state.LoadStateTree(req.Context, fcn.CborStore, blk.StateRoot, builtin.Actors)
		if err != nil {
			return err
		}

		addr, err := types.NewAddressFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		act, err := tree.GetActor(req.Context, addr)
		if err != nil {
			if state.IsActorNotFoundError(err) {
				// if the account doesn't exit, the balance should be zero
				re.Emit(types.NewAttoFILFromFIL(0)) // nolint: errcheck
				return nil
			}
			return err
		}

		re.Emit(act.Balance) // nolint: errcheck
		return nil
	},
	Type: &types.AttoFIL{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, b *types.AttoFIL) error {
			return PrintString(w, b)
		}),
	},
}
