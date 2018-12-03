package commands

import (
	"fmt"
	"io"

	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		target, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		amount, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			re.SetError(ErrInvalidAmount, cmdkit.ErrNormal)
			return
		}

		eol, ok := types.NewBlockHeightFromString(req.Arguments[2], 10)
		if !ok {
			re.SetError(ErrInvalidBlockHeight, cmdkit.ErrNormal)
			return
		}

		c, err := GetAPI(env).Paych().Create(req.Context, fromAddr, target, eol, amount)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(c) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		payerOption := req.Options["payer"]
		payerAddr, err := optionalAddr(payerOption)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		channels, err := GetAPI(env).Paych().Ls(req.Context, fromAddr, payerAddr)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(channels) // nolint: errcheck
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			re.SetError("invalid channel id", cmdkit.ErrNormal)
			return
		}

		amount, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			re.SetError(ErrInvalidAmount, cmdkit.ErrNormal)
			return
		}

		voucher, err := GetAPI(env).Paych().Voucher(req.Context, fromAddr, channel, amount)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(voucher) // nolint: errcheck
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		c, err := GetAPI(env).Paych().Redeem(req.Context, fromAddr, req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(c) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			re.SetError("invalid channel id", cmdkit.ErrNormal)
			return
		}

		c, err := GetAPI(env).Paych().Reclaim(req.Context, fromAddr, channel)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(c) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		c, err := GetAPI(env).Paych().Close(req.Context, fromAddr, req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(c) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fromAddr, err := optionalAddr(req.Options["from"])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			re.SetError("invalid channel id", cmdkit.ErrNormal)
			return
		}

		amount, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			re.SetError(ErrInvalidAmount, cmdkit.ErrNormal)
			return
		}

		eol, ok := types.NewBlockHeightFromString(req.Arguments[2], 10)
		if !ok {
			re.SetError(ErrInvalidBlockHeight, cmdkit.ErrNormal)
			return
		}

		c, err := GetAPI(env).Paych().Extend(req.Context, fromAddr, channel, eol, amount)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(c) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			fmt.Fprintln(w, c) // nolint: errcheck
			return nil
		}),
	},
}
