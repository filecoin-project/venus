package commands

import (
	"fmt"
	"io"
	"math/big"
	"strconv"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/address"
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
		"update-peerid": minerUpdatePeerIDCmd,
	},
}

var minerPledgeCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "View number of pledged 1GB sectors for <miner>",
		ShortDescription: `Shows the number of pledged 1GB sectors for the given miner address`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "the miner address"),
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

var minerCreateCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create a new file miner with <pledge> 1GB sectors and <collateral> FIL",
		ShortDescription: `Issues a new message to the network to create the miner, then waits for the
message to be mined as this is required to return the address of the new miner.
Collateral must be greater than 0.001 FIL per pledged sector.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("pledge", true, false, "the size of the pledge (in 1GB sectors) for the miner"),
		cmdkit.StringArg("collateral", true, false, "the amount of collateral in FIL to be sent (minimum 0.001 FIL per sector)"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send from"),
		cmdkit.StringOption("peerid", "b58-encoded libp2p peer ID that the miner will operate"),
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

		addr, err := GetAPI(env).Miner().Create(req.Context, fromAddr, pledge, pid, collateral)
		if err != nil {
			return err
		}

		re.Emit(&addr) // nolint: errcheck
		return nil
	},
	Type: address.Address{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *address.Address) error {
			return PrintString(w, a)
		}),
	},
}

var minerUpdatePeerIDCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Change the libp2p identity that a miner is operating",
		ShortDescription: `Issues a new message to the network to update the miner's libp2p identity.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, false, "miner address to update peer ID for"),
		cmdkit.StringArg("peerid", true, false, "b58-encoded libp2p peer ID that the miner will operate"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send from"),
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

		c, err := GetAPI(env).Miner().UpdatePeerID(req.Context, fromAddr, minerAddr, newPid)
		if err != nil {
			return err
		}

		re.Emit(c) // nolint: errcheck
		return nil
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			return PrintString(w, c)
		}),
	},
}

var minerAddAskCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Add an ask to the storage market",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "the address of the miner owning the ask"),
		cmdkit.StringArg("price", true, false, "the price in FIL of the ask"),
		cmdkit.StringArg("expiry", true, false, "how long this ask is valid for in blocks"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send the ask from"),
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
			return errors.New("expiry must be a valid integer")
		}

		c, err := GetAPI(env).Miner().AddAsk(req.Context, fromAddr, minerAddr, price, expiry)
		if err != nil {
			return err
		}
		re.Emit(c) // nolint: errcheck
		return nil
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			return PrintString(w, c)
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

		re.Emit(&ownerAddr) // nolint: errcheck
		return nil
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "the address of the miner"),
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
		re.Emit(str) // nolint: errcheck
		return nil
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "the address of the miner"),
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a string) error {
			_, err := fmt.Fprintln(w, a)
			return err
		}),
	},
}
