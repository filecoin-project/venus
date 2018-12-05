package commands

import (
	"errors"
	"fmt"
	"io"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

var paymentChannelCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Payment channel operations",
	},
	Subcommands: map[string]*cmds.Command{
		"close":   closeCmd,
		"create":  createChannelCmd,
		"extend":  extendCmd,
		"ls":      lsCmd,
		"reclaim": reclaimCmd,
		"redeem":  redeemCmd,
		"voucher": voucherCmd,
	},
}

var createChannelCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create a new payment channel",
		ShortDescription: `Issues a new message to the network to create a payment channeld. Then waits for the
message to be mined to get the channelID.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("target", true, false, "address of account that will redeem funds"),
		cmdkit.StringArg("amount", true, false, "amount in FIL for the channel"),
		cmdkit.StringArg("eol", true, false, "the block height at which the channel should expire"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send from"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			return err
		}

		target, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		amount, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			return ErrInvalidAmount
		}

		eol, ok := types.NewBlockHeightFromString(req.Arguments[2], 10)
		if !ok {
			return ErrInvalidBlockHeight
		}

		c, err := GetAPI(env).Paych().Create(req.Context, fromAddr, target, eol, amount)
		if err != nil {
			return err
		}

		re.Emit(c) // nolint: errcheck
		return nil
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c cid.Cid) error {
			return PrintString(w, c)
		}),
	},
}

var lsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "List all payment channels for a payer",
		ShortDescription: `Queries the payment broker to find all payment channels where a given account is the payer.`,
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address for which message is sent"),
		cmdkit.StringOption("payer", "address for which to retrieve channels (defaults to from if omitted)"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			return err
		}

		payerOption := req.Options["payer"]
		payerAddr, err := optionalAddr(payerOption)
		if err != nil {
			return err
		}

		channels, err := GetAPI(env).Paych().Ls(req.Context, fromAddr, payerAddr)
		if err != nil {
			return err
		}

		re.Emit(channels) // nolint: errcheck
		return nil
	},
	Type: map[string]*paymentbroker.PaymentChannel{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, pcs *map[string]*paymentbroker.PaymentChannel) error {
			if len(*pcs) == 0 {
				fmt.Fprintln(w, "no channels") // nolint: errcheck
				return nil
			}

			for chid, pc := range *pcs {
				_, err := fmt.Fprintf(w, "%s: target: %v, amt: %v, amt redeemed: %v, eol: %v\n", chid, pc.Target.String(), pc.Amount, pc.AmountRedeemed, pc.Eol)
				if err != nil {
					return err
				}
			}
			return nil
		}),
	},
}

var voucherCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Create a new voucher from a payment channel",
		ShortDescription: `Generate a new signed payment voucher for the target of a payment channel.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("channel", true, false, "channel id of channel from which to create voucher"),
		cmdkit.StringArg("amount", true, false, "amount in FIL of this voucher"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address for which to retrieve channels"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			return err
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			return errors.New("invalid channel id")
		}

		amount, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			return ErrInvalidAmount
		}

		voucher, err := GetAPI(env).Paych().Voucher(req.Context, fromAddr, channel, amount)
		if err != nil {
			return err
		}

		re.Emit(voucher) // nolint: errcheck
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, voucher string) error {
			fmt.Fprintln(w, voucher) // nolint: errcheck
			return nil
		}),
	},
}

var redeemCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Redeem a payment voucher against a payment channel",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("voucher", true, false, "base58 encoded signed voucher"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address of the channel target"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			return err
		}

		c, err := GetAPI(env).Paych().Redeem(req.Context, fromAddr, req.Arguments[0])
		if err != nil {
			return err
		}

		re.Emit(c) // nolint: errcheck
		return nil
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c cid.Cid) error {
			return PrintString(w, c)
		}),
	},
}

var reclaimCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Reclaim funds from an expired channel",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("channel", true, false, "id of channel from which funds are reclaimed"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address of the channel creator"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			return err
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			return errors.New("invalid channel id")
		}

		c, err := GetAPI(env).Paych().Reclaim(req.Context, fromAddr, channel)
		if err != nil {
			return err
		}

		re.Emit(c) // nolint: errcheck
		return nil
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c cid.Cid) error {
			fmt.Fprintln(w, c) // nolint: errcheck
			return nil
		}),
	},
}

var closeCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Redeem a payment voucher and close the payment channel",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("voucher", true, false, "base58 encoded signed voucher"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address of the channel target"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			return err
		}

		c, err := GetAPI(env).Paych().Close(req.Context, fromAddr, req.Arguments[0])
		if err != nil {
			return err
		}

		re.Emit(c) // nolint: errcheck
		return nil
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c cid.Cid) error {
			return PrintString(w, c)
		}),
	},
}

var extendCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Extend the value and lifetime of a given channel",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("channel", true, false, "id of channel to extend"),
		cmdkit.StringArg("amount", true, false, "amount in FIL for the channel"),
		cmdkit.StringArg("eol", true, false, "the block height at which the channel should expire"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address of the channel creator"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			return err
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			return errors.New("invalid channel id")
		}

		amount, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			return ErrInvalidAmount
		}

		eol, ok := types.NewBlockHeightFromString(req.Arguments[2], 10)
		if !ok {
			return ErrInvalidBlockHeight
		}

		c, err := GetAPI(env).Paych().Extend(req.Context, fromAddr, channel, eol, amount)
		if err != nil {
			return err
		}

		re.Emit(c) // nolint: errcheck
		return nil
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c cid.Cid) error {
			fmt.Fprintln(w, c) // nolint: errcheck
			return nil
		}),
	},
}
