package commands

import (
	"io"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

var minerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage miner operations",
	},
	Subcommands: map[string]*cmds.Command{
		"create":  minerCreateCmd,
		"add-ask": minerAddAskCmd,
	},
}

var minerCreateCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create a new file miner",
		ShortDescription: `Issues a new message to the network to create the miner. Then waits for the
message to be mined as this is required to return the address of the new miner.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("pledge", true, false, "the size of the pledge for the miner"),
		cmdkit.StringArg("collateral", true, false, "the amount of collateral to be sent"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send from"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n := GetNode(env)

		fromAddr, err := types.NewAddressFromString(req.Options["from"].(string))
		if err != nil {
			return errors.Wrap(err, "invalid from address")
		}

		pledge, ok := types.NewBytesAmountFromString(req.Arguments[0], 10)
		if !ok {
			return ErrInvalidPledge
		}

		collateral, ok := types.NewTokenAmountFromString(req.Arguments[1], 10)
		if !ok {
			return ErrInvalidCollateral
		}

		params, err := abi.ToEncodedValues(pledge)
		if err != nil {
			return err
		}

		msg := types.NewMessage(fromAddr, core.StorageMarketAddress, collateral, "createMiner", params)
		if err := n.AddNewMessage(req.Context, msg); err != nil {
			return err
		}

		msgCid, err := msg.Cid()
		if err != nil {
			return err
		}

		re.Emit(msgCid) // nolint: errcheck

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
		cmdkit.StringArg("size", true, false, "size in bytes of the ask"),
		cmdkit.StringArg("price", true, false, "the price of the ask"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send the ask from"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n := GetNode(env)

		fromAddr, err := types.NewAddressFromString(req.Options["from"].(string))
		if err != nil {
			return errors.Wrap(err, "invalid from address")
		}

		minerAddr, err := types.NewAddressFromString(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid miner address")
		}

		size, ok := types.NewBytesAmountFromString(req.Arguments[1], 10)
		if !ok {
			return ErrInvalidSize
		}

		price, ok := types.NewTokenAmountFromString(req.Arguments[2], 10)
		if !ok {
			return ErrInvalidPrice
		}

		params, err := abi.ToEncodedValues(price, size)
		if err != nil {
			return err
		}

		msg := types.NewMessage(fromAddr, minerAddr, nil, "addAsk", params)
		if err := n.AddNewMessage(req.Context, msg); err != nil {
			return err
		}

		c, err := msg.Cid()
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
