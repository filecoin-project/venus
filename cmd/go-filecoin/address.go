package commands

import (
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/internal/pkg/types"
	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
)

var walletCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manage your filecoin wallets",
	},
	Subcommands: map[string]*cmds.Command{
		"balance": balanceCmd,
		"import":  walletImportCmd,
		"export":  walletExportCmd,
	},
}

var addrsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with addresses",
	},
	Subcommands: map[string]*cmds.Command{
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
		addr, err := GetPorcelainAPI(env).WalletNewAddress(protocol)
		if err != nil {
			return err
		}
		return re.Emit(&AddressResult{addr})
	},
	Options: []cmds.Option{
		cmds.StringOption("type", "The type of address to create: bls or secp256k1 (default)").WithDefault("secp256k1"),
	},
	Type: &AddressResult{},
}

var addrsLsCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addrs := GetPorcelainAPI(env).WalletAddresses()

		var alr AddressLsResult
		for _, addr := range addrs {
			alr.Addresses = append(alr.Addresses, addr)
		}

		return re.Emit(&alr)
	},
	Type: &AddressLsResult{},
}

var defaultAddressCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addr, err := GetPorcelainAPI(env).WalletDefaultAddress()
		if err != nil {
			return err
		}

		return re.Emit(&AddressResult{addr})
	},
	Type: &AddressResult{},
}

var setDefaultAddressCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address to set default for"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		err = GetPorcelainAPI(env).SetWalletDefaultAddress(addr)
		if err != nil {
			return err
		}

		return re.Emit(&AddressResult{addr})
	},
	Type: &AddressResult{},
}

var balanceCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address to get balance for"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		balance, err := GetPorcelainAPI(env).WalletBalance(req.Context, addr)
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

		var wir *WalletSerializeResult
		if err := json.NewDecoder(fi).Decode(&wir); err != nil {
			return err
		}
		keyInfos := wir.KeyInfo

		if len(keyInfos) == 0 {
			return fmt.Errorf("no keys in wallet file")
		}

		addrs, err := GetPorcelainAPI(env).WalletImport(keyInfos...)
		if err != nil {
			return err
		}

		var alr AddressLsResult
		for _, addr := range addrs {
			alr.Addresses = append(alr.Addresses, addr)
		}

		return re.Emit(&alr)
	},
	Type: &AddressLsResult{},
}

var walletExportCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.StringArg("addresses", true, true, "Addresses of keys to export").EnableStdin(),
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

		kis, err := GetPorcelainAPI(env).WalletExport(addrs)
		if err != nil {
			return err
		}

		var klr WalletSerializeResult
		klr.KeyInfo = append(klr.KeyInfo, kis...)

		return re.Emit(klr)
	},
	Type: &WalletSerializeResult{},
}
