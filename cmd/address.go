package cmd

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/filecoin-project/go-address"
	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/cmd/tablewriter"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/types"
)

var walletCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manage your filecoin wallets",
	},
	Subcommands: map[string]*cmds.Command{
		"balance":     balanceCmd,
		"import":      walletImportCmd,
		"export":      walletExportCmd,
		"ls":          addrsLsCmd,
		"new":         addrsNewCmd,
		"default":     defaultAddressCmd,
		"set-default": setDefaultAddressCmd,
	},
}

type AddressResult struct {
	Address address.Address
}

// AddressLsResult is the result of running the address list command.
type AddressLsResult struct {
	Addresses []address.Address
}

var addrsNewCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		protocolName := req.Options["type"].(string)
		var protocol address.Protocol
		switch protocolName {
		case "secp256k1":
			protocol = address.SECP256K1
		case "bls":
			protocol = address.BLS
		default:
			return fmt.Errorf("unrecognized address protocol %s", protocolName)
		}

		addr, err := env.(*node.Env).WalletAPI.WalletNewAddress(protocol)
		if err != nil {
			return err
		}

		return printOneString(re, addr.String())
	},
	Options: []cmds.Option{
		cmds.StringOption("type", "The type of address to create: bls (default) or secp256k1").WithDefault("bls"),
	},
}

var addrsLsCmd = &cmds.Command{
	Options: []cmds.Option{
		cmds.BoolOption("addr-only", "Only print addresses"),
		cmds.BoolOption("id", "Output ID addresses"),
		cmds.BoolOption("market", "Output market balances"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		api := env.(*node.Env)
		ctx := req.Context

		addrs := api.WalletAPI.WalletAddresses()

		// Assume an error means no default key is set
		def, _ := api.WalletAPI.WalletDefaultAddress()

		buf := new(bytes.Buffer)
		tw := tablewriter.New(
			tablewriter.Col("Address"),
			tablewriter.Col("ID"),
			tablewriter.Col("Balance"),
			tablewriter.Col("Market(Avail)"),
			tablewriter.Col("Market(Locked)"),
			tablewriter.Col("Nonce"),
			tablewriter.Col("Default"),
			tablewriter.NewLineCol("Error"))

		addrOnly := false
		if _, ok := req.Options["addr-only"]; ok {
			addrOnly = true
		}
		for _, addr := range addrs {
			if addrOnly {
				writer := NewSilentWriter(buf)
				writer.WriteStringln(addr.String())
			} else {
				a, err := api.ChainAPI.StateGetActor(ctx, addr, block.EmptyTSK)
				if err != nil {
					if !strings.Contains(err.Error(), "actor not found") {
						tw.Write(map[string]interface{}{
							"Address": addr,
							"Error":   err,
						})
						continue
					}

					a = &types.Actor{
						Balance: big.Zero(),
					}
				}

				row := map[string]interface{}{
					"Address": addr,
					"Balance": types.FIL(a.Balance),
					"Nonce":   a.Nonce,
				}
				if addr == def {
					row["Default"] = "X"
				}

				if _, ok := req.Options["id"]; ok {
					id, err := api.ChainAPI.StateLookupID(ctx, addr, block.EmptyTSK)
					if err != nil {
						row["ID"] = "n/a"
					} else {
						row["ID"] = id
					}
				}

				if _, ok := req.Options["market"]; ok {
					mbal, err := api.ChainAPI.StateMarketBalance(ctx, addr, block.EmptyTSK)
					if err == nil {
						row["Market(Avail)"] = types.FIL(crypto.BigSub(mbal.Escrow, mbal.Locked))
						row["Market(Locked)"] = types.FIL(mbal.Locked)
					}
				}

				tw.Write(row)
			}
		}

		if !addrOnly {
			_ = tw.Flush(buf)
		}

		return re.Emit(buf)
	},
}

var defaultAddressCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addr, err := env.(*node.Env).WalletAPI.WalletDefaultAddress()
		if err != nil {
			return err
		}

		return printOneString(re, addr.String())
	},
}

var setDefaultAddressCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "address to set default for"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		err = env.(*node.Env).WalletAPI.WalletSetDefault(context.TODO(), addr)
		if err != nil {
			return err
		}

		return printOneString(re, addr.String())
	},
}

var balanceCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "APIAddress to get balance for"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		balance, err := env.(*node.Env).WalletAPI.WalletBalance(req.Context, addr)
		if err != nil {
			return err
		}
		return re.Emit(balance)
	},
	Type: &types.AttoFIL{},
}

// WalletSerializeResult is the type wallet export and import return and expect.
type WalletSerializeResult struct {
	KeyInfo []*crypto.KeyInfo
}

var walletImportCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.FileArg("walletFile", true, false, "File containing wallet data to import").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		iter := req.Files.Entries()
		if !iter.Next() {
			return fmt.Errorf("no file given: %s", iter.Err())
		}

		fi, ok := iter.Node().(files.File)
		if !ok {
			return fmt.Errorf("given file was not a files.File")
		}

		var key crypto.KeyInfo
		if err := json.NewDecoder(hex.NewDecoder(fi)).Decode(&key); err != nil {
			return err
		}

		addr, err := env.(*node.Env).WalletAPI.WalletImport(&key)
		if err != nil {
			return err
		}

		return printOneString(re, addr.String())
	},
}

var walletExportCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.StringArg("addr", true, true, "address of keys to export").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addrs := make([]address.Address, len(req.Arguments))
		for i, arg := range req.Arguments {
			addr, err := address.NewFromString(arg)
			if err != nil {
				return err
			}
			addrs[i] = addr
		}

		kis, err := env.(*node.Env).WalletAPI.WalletExport(addrs)
		if err != nil {
			return err
		}
		data, err := json.Marshal(kis[0])
		if err != nil {
			return err
		}
		return printOneString(re, hex.EncodeToString(data))
	},
}
