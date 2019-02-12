package commands

import (
	"fmt"
	"io"
	"math/big"
	"strconv"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

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
		"add-ask":       minerAddAskCmd,
		"owner":         minerOwnerCmd,
		"pledge":        minerPledgeCmd,
		"power":         minerPowerCmd,
		"set-price":     minerSetPriceCmd,
		"update-peerid": minerUpdatePeerIDCmd,
	},
}

var minerPledgeCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "View number of pledged 1GB sectors for <miner>",
		ShortDescription: `Shows the number of pledged 1GB sectors for the given miner address`,
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
		pledgeSectors, err := GetAPI(env).Miner().GetPledge(req.Context, minerAddr)
		if err != nil {
			return err
		}

		str := fmt.Sprintf("%d", pledgeSectors)
		re.Emit(str) // nolint: errcheck
		return nil
	},
}

type minerCreateResult struct {
	Address address.Address
	GasUsed types.GasUnits
	Preview bool
}

var minerCreateCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create a new file miner with <pledge> 1GB sectors and <collateral> FIL",
		ShortDescription: `Issues a new message to the network to create the miner, then waits for the
message to be mined as this is required to return the address of the new miner.
Collateral must be greater than 0.001 FIL per pledged sector.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("pledge", true, false, "The size of the pledge (in 1GB sectors) for the miner"),
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
			return re.Emit(&minerCreateResult{
				Address: address.Address{},
				GasUsed: usedGas,
				Preview: true,
			})
		}

		addr, err := GetAPI(env).Miner().Create(req.Context, fromAddr, gasPrice, gasLimit, pledge, pid, collateral)
		if err != nil {
			return err
		}

		return re.Emit(&minerCreateResult{
			Address: addr,
			GasUsed: types.NewGasUnits(0),
			Preview: false,
		})
	},
	Type: &minerCreateResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *minerCreateResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Address)
		}),
	},
}

type minerSetPriceResult struct {
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
		cmdkit.StringArg("storageprice", true, false, "The new price of storage per sector in FIL"),
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
			return re.Emit(&minerSetPriceResult{
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

		return re.Emit(&minerSetPriceResult{
			GasUsed:               types.NewGasUnits(0),
			Preview:               false,
			MinerSetPriceResponse: res,
		})
	},
	Type: &minerSetPriceResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *minerSetPriceResult) error {
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

type minerUpdatePeerIDResult struct {
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

			return re.Emit(&minerUpdatePeerIDResult{
				Cid:     cid.Cid{},
				GasUsed: usedGas,
				Preview: true,
			})
		}

		c, err := GetAPI(env).Miner().UpdatePeerID(req.Context, fromAddr, minerAddr, gasPrice, gasLimit, newPid)
		if err != nil {
			return err
		}

		return re.Emit(&minerUpdatePeerIDResult{
			Cid:     c,
			GasUsed: types.NewGasUnits(0),
			Preview: false,
		})
	},
	Type: &minerUpdatePeerIDResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *minerUpdatePeerIDResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Cid)
		}),
	},
}

type minerAddAskResult struct {
	Cid     cid.Cid
	GasUsed types.GasUnits
	Preview bool
}

var minerAddAskCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "DEPRECATED: Use set-price",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "The address of the miner owning the ask"),
		cmdkit.StringArg("price", true, false, "The price in FIL of the ask"),
		cmdkit.StringArg("expiry", true, false, "How long this ask is valid for in blocks"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address to send the ask from"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			return err
		}

		minerAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid miner address")
		}

		price, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			return ErrInvalidPrice
		}

		expiry, ok := big.NewInt(0).SetString(req.Arguments[2], 10)
		if !ok {
			return fmt.Errorf("expiry must be a valid integer")
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
				"addAsk",
				price,
				expiry,
			)
			if err != nil {
				return err
			}
			return re.Emit(&minerAddAskResult{
				Cid:     cid.Cid{},
				GasUsed: usedGas,
				Preview: true,
			})
		}

		c, err := GetAPI(env).Miner().AddAsk(req.Context, fromAddr, minerAddr, gasPrice, gasLimit, price, expiry)
		if err != nil {
			return err
		}
		return re.Emit(&minerAddAskResult{
			Cid:     c,
			GasUsed: types.NewGasUnits(0),
			Preview: false,
		})
	},
	Type: &minerAddAskResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *minerAddAskResult) error {
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
		ownerAddr, err := GetAPI(env).Miner().GetOwner(req.Context, minerAddr)
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
		power, err := GetAPI(env).Miner().GetPower(req.Context, minerAddr)
		if err != nil {
			return err
		}
		total, err := GetAPI(env).Miner().GetTotalPower(req.Context)
		if err != nil {
			return err
		}

		str := fmt.Sprintf("%d / %d", power, total)
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
