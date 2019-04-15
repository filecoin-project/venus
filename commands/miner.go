package commands

import (
	"fmt"
	"io"
	"math/big"
	"strconv"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/types"
)

var minerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage a single miner actor",
	},
	Subcommands: map[string]*cmds.Command{
		"create":        minerCreateCmd,
		"owner":         minerOwnerCmd,
		"pledge":        minerPledgeCmd,
		"power":         minerPowerCmd,
		"set-price":     minerSetPriceCmd,
		"update-peerid": minerUpdatePeerIDCmd,
	},
}

var minerPledgeCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "View number of pledged sectors for <miner>",
		ShortDescription: `Shows the number of pledged sectors for the given miner address`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "The miner address"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		var err error

		minerAddr, err := optionalAddr(req.Arguments[0])
		if err != nil {
			return err
		}

		bytes, err := GetPorcelainAPI(env).MessageQuery(
			req.Context,
			address.Undef,
			minerAddr,
			"getPledge",
		)
		if err != nil {
			return err
		}
		pledgeSectors := big.NewInt(0).SetBytes(bytes[0])

		str := fmt.Sprintf("%d", pledgeSectors) // nolint: govet
		return re.Emit(str)
	},
}

// MinerCreateResult is the type returned when creating a miner.
type MinerCreateResult struct {
	Address address.Address
	GasUsed types.GasUnits
	Preview bool
}

var minerCreateCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create a new file miner with <pledge> sectors and <collateral> FIL",
		ShortDescription: `Issues a new message to the network to create the miner, then waits for the
message to be mined as this is required to return the address of the new miner.
Collateral must be greater than 0.001 FIL per pledged sector.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("pledge", true, false, "The size of the pledge (in sectors) for the miner"),
		cmdkit.StringArg("collateral", true, false, "The amount of collateral in FIL to be sent (minimum 0.001 FIL per sector)"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address to send from"),
		cmdkit.StringOption("peerid", "Base58-encoded libp2p peer ID that the miner will operate"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		var err error

		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			return err
		}

		var pid peer.ID
		peerid := req.Options["peerid"]
		if peerid != nil {
			pid, err = peer.IDB58Decode(peerid.(string))
			if err != nil {
				return errors.Wrap(err, "invalid peer id")
			}
		}
		if pid == "" {
			pid = GetPorcelainAPI(env).NetworkGetPeerID()
		}

		pledge, err := strconv.ParseUint(req.Arguments[0], 10, 64)
		if err != nil {
			return ErrInvalidPledge
		}

		collateral, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			return ErrInvalidCollateral
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		if preview {
			usedGas, err := GetPorcelainAPI(env).MinerPreviewCreate(
				req.Context,
				fromAddr,
				pledge,
				pid,
				collateral,
			)
			if err != nil {
				return err
			}
			return re.Emit(&MinerCreateResult{
				Address: address.Undef,
				GasUsed: usedGas,
				Preview: true,
			})
		}

		addr, err := GetPorcelainAPI(env).MinerCreate(
			req.Context,
			fromAddr,
			gasPrice,
			gasLimit,
			pledge,
			pid,
			collateral,
		)
		if err != nil {
			return errors.Wrap(err, "Could not create miner. Please consult the documentation to setup your wallet and genesis block correctly")
		}

		return re.Emit(&MinerCreateResult{
			Address: *addr,
			GasUsed: types.NewGasUnits(0),
			Preview: false,
		})
	},
	Type: &MinerCreateResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MinerCreateResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Address)
		}),
	},
}

// MinerSetPriceResult is the return type for miner set-price command
type MinerSetPriceResult struct {
	GasUsed               types.GasUnits
	MinerSetPriceResponse porcelain.MinerSetPriceResponse
	Preview               bool
}

var minerSetPriceCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Set the minimum price for storage",
		ShortDescription: `Sets the mining.minimumPrice in config and creates a new ask for the given price.
This command waits for the ask to be mined.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("storageprice", true, false, "The new price of storage in FIL per byte per block"),
		cmdkit.StringArg("expiry", true, false, "How long this ask is valid for in blocks"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address to send from"),
		cmdkit.StringOption("miner", "The address of the miner owning the ask"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		price, ok := types.NewAttoFILFromFILString(req.Arguments[0])
		if !ok {
			return ErrInvalidPrice
		}

		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			return err
		}

		var minerAddr address.Address
		if req.Options["miner"] != nil {
			minerAddr, err = address.NewFromString(req.Options["miner"].(string))
			if err != nil {
				return errors.Wrap(err, "miner must be an address")
			}
		}

		expiry, ok := big.NewInt(0).SetString(req.Arguments[1], 10)
		if !ok {
			return fmt.Errorf("expiry must be a valid integer")
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		if preview {
			usedGas, err := GetPorcelainAPI(env).MinerPreviewSetPrice(
				req.Context,
				fromAddr,
				minerAddr,
				price,
				expiry)
			if err != nil {
				return err
			}
			return re.Emit(&MinerSetPriceResult{
				GasUsed:               usedGas,
				Preview:               true,
				MinerSetPriceResponse: porcelain.MinerSetPriceResponse{},
			})
		}

		res, err := GetPorcelainAPI(env).MinerSetPrice(
			req.Context,
			fromAddr,
			minerAddr,
			gasPrice,
			gasLimit,
			price,
			expiry)
		if err != nil {
			return err
		}

		return re.Emit(&MinerSetPriceResult{
			GasUsed:               types.NewGasUnits(0),
			Preview:               false,
			MinerSetPriceResponse: res,
		})
	},
	Type: &MinerSetPriceResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MinerSetPriceResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			_, err := fmt.Fprintf(w, `Set price for miner %s to %s.
	Published ask, cid: %s.
	Ask confirmed on chain in block: %s.
	`,
				res.MinerSetPriceResponse.MinerAddr.String(),
				res.MinerSetPriceResponse.Price.String(),
				res.MinerSetPriceResponse.AddAskCid.String(),
				res.MinerSetPriceResponse.BlockCid.String(),
			)
			return err
		}),
	},
}

// MinerUpdatePeerIDResult is the return type for miner update-peerid command
type MinerUpdatePeerIDResult struct {
	Cid     cid.Cid
	GasUsed types.GasUnits
	Preview bool
}

var minerUpdatePeerIDCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Change the libp2p identity that a miner is operating",
		ShortDescription: `Issues a new message to the network to update the miner's libp2p identity.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, false, "Miner address to update peer ID for"),
		cmdkit.StringArg("peerid", true, false, "Base58-encoded libp2p peer ID that the miner will operate"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address to send from"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			return err
		}

		newPid, err := peer.IDB58Decode(req.Arguments[1])
		if err != nil {
			return err
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		if preview {
			usedGas, err := GetPorcelainAPI(env).MessagePreview(
				req.Context,
				fromAddr,
				minerAddr,
				"updatePeerID",
				newPid,
			)
			if err != nil {
				return err
			}

			return re.Emit(&MinerUpdatePeerIDResult{
				Cid:     cid.Cid{},
				GasUsed: usedGas,
				Preview: true,
			})
		}

		c, err := GetPorcelainAPI(env).MessageSendWithDefaultAddress(
			req.Context,
			fromAddr,
			minerAddr,
			nil,
			gasPrice,
			gasLimit,
			"updatePeerID",
			newPid,
		)
		if err != nil {
			return err
		}

		return re.Emit(&MinerUpdatePeerIDResult{
			Cid:     c,
			GasUsed: types.NewGasUnits(0),
			Preview: false,
		})
	},
	Type: &MinerUpdatePeerIDResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MinerUpdatePeerIDResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Cid)
		}),
	},
}

var minerOwnerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Show the actor address of <miner>",
		ShortDescription: `Given <miner> miner address, output the address of the actor that owns the miner.`,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddr, err := optionalAddr(req.Arguments[0])
		if err != nil {
			return err
		}

		bytes, err := GetPorcelainAPI(env).MessageQuery(
			req.Context,
			address.Undef,
			minerAddr,
			"getOwner",
		)
		if err != nil {
			return err
		}
		ownerAddr, err := address.NewFromBytes(bytes[0])
		if err != nil {
			return err
		}

		return re.Emit(&ownerAddr)
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "The address of the miner"),
	},
	Type: address.Address{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *address.Address) error {
			return PrintString(w, a)
		}),
	},
}

var minerPowerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get the power of a miner versus the total storage market power",
		ShortDescription: `Check the current power of a given miner and total power of the storage market.
Values will be output as a ratio where the first number is the miner power and second is the total market power.`,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddr, err := optionalAddr(req.Arguments[0])
		if err != nil {
			return err
		}

		bytes, err := GetPorcelainAPI(env).MessageQuery(
			req.Context,
			address.Undef,
			minerAddr,
			"getPower",
		)
		if err != nil {
			return err
		}
		power := big.NewInt(0).SetBytes(bytes[0])

		bytes, err = GetPorcelainAPI(env).MessageQuery(
			req.Context,
			address.Undef,
			address.StorageMarketAddress,
			"getTotalStorage",
		)
		if err != nil {
			return err
		}
		total := big.NewInt(0).SetBytes(bytes[0])

		str := fmt.Sprintf("%d / %d", power, total) // nolint: govet
		return re.Emit(str)
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "The address of the miner"),
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a string) error {
			_, err := fmt.Fprintln(w, a)
			return err
		}),
	},
}
