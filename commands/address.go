package commands

import (
	"encoding/json"
	"fmt"
	"io"

	cmds "gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit/files"

	"github.com/filecoin-project/go-filecoin/node/impl"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var walletCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage your filecoin wallets",
	},
	Subcommands: map[string]*cmds.Command{
		"addrs":   addrsCmd,
		"balance": balanceCmd,
		"import":  walletImportCmd,
		"export":  walletExportCmd,
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)
		addr, err := fcn.NewAddress()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(&addressResult{addr.String()}) // nolint: errcheck
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)
		for _, a := range fcn.Wallet.Addresses() {
			re.Emit(&addressResult{a.String()}) // nolint: errcheck
		}
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)

		address, err := types.NewAddressFromString(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		v, err := fcn.Lookup.GetPeerIDByMinerAddress(req.Context, address)
		if err != nil {
			err = errors.Wrapf(err, "failed to find miner with address %s", address.String())
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		re.Emit(v.Pretty()) // nolint: errcheck
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)
		ts := fcn.ChainMgr.GetHeaviestTipSet()
		if len(ts) == 0 {
			re.SetError(impl.ErrHeaviestTipSetNotFound, cmdkit.ErrNormal)
			return
		}

		tree, _, err := fcn.ChainMgr.State(req.Context, ts.ToSlice())
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		addr, err := types.NewAddressFromString(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		act, err := tree.GetActor(req.Context, addr)
		if err != nil {
			if state.IsActorNotFoundError(err) {
				// if the account doesn't exit, the balance should be zero
				re.Emit(types.NewAttoFILFromFIL(0)) // nolint: errcheck
				return
			}
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(act.Balance) // nolint: errcheck
	},
	Type: &types.AttoFIL{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, b *types.AttoFIL) error {
			return PrintString(w, b)
		}),
	},
}

var walletImportCmd = &cmds.Command{
	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("walletFile", true, false, "file containing wallet data to import").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)

		kinfos, err := parseKeyInfos(req.Files)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		dsb := fcn.Wallet.Backends(wallet.DSBackendType)
		if len(dsb) != 1 {
			re.SetError("expected exactly one datastore wallet backend", cmdkit.ErrNormal)
			return
		}

		imp, ok := dsb[0].(wallet.Importer)
		if !ok {
			re.SetError("datastore backend wallets should implement importer", cmdkit.ErrNormal)
			return
		}

		for _, ki := range kinfos {
			if err := imp.ImportKey(ki); err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
		}
	},
}

var walletExportCmd = &cmds.Command{
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("addresses", true, true, "addresses of keys to export").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)

		for _, arg := range req.Arguments {
			addr, err := types.NewAddressFromString(arg)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			bck, err := fcn.Wallet.Find(addr)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			ki, err := bck.GetKeyInfo(addr)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			re.Emit(ki) // nolint: errcheck
		}
	},
	Type: types.KeyInfo{},
}

func parseKeyInfos(f files.File) ([]*types.KeyInfo, error) {
	var kinfos []*types.KeyInfo
	for {
		fi, err := f.NextFile()
		switch err {
		case io.EOF:
			return kinfos, nil
		default:
			return nil, err
		case nil:
		}

		var ki types.KeyInfo
		if err := json.NewDecoder(fi).Decode(&ki); err != nil {
			return nil, err
		}

		kinfos = append(kinfos, &ki)
	}
}
