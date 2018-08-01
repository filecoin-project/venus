package commands

import (
	"fmt"
	"io"

	cbor "gx/ipfs/QmSyK1ZiAP98YvnxsTfQpb669V2xeTHRbG4Y6fgKS3vVSd/go-ipld-cbor"
	"gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
	"gx/ipfs/QmexBtiTTEwwn42Yi6ouKt6VqzpA6wjJgiW1oh9VfaRrup/go-multibase"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
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
		cmdkit.StringArg("amount", true, false, "filecoin amount for the channel"),
		cmdkit.StringArg("eol", true, false, "the block height at which the channel should expire"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address to send from"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		target, err := types.NewAddressFromString(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		amount, ok := types.NewAttoFILFromFILString(req.Arguments[1], 10)
		if !ok {
			re.SetError(ErrInvalidAmount, cmdkit.ErrNormal)
			return
		}

		eol, ok := types.NewBlockHeightFromString(req.Arguments[2], 10)
		if !ok {
			re.SetError(ErrInvalidBlockHeight, cmdkit.ErrNormal)
			return
		}

		params, err := abi.ToEncodedValues(target, eol)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		// TODO: Sign this message
		msg, err := node.NewMessageWithNextNonce(req.Context, n, fromAddr, address.PaymentBrokerAddress, amount, "createChannel", params)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if err := n.AddNewMessage(req.Context, msg); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		msgCid, err := msg.Cid()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(msgCid) // nolint: errcheck
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
		n := GetNode(env)

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		var payer types.Address
		payerOption := req.Options["payer"]
		if payerOption != nil {
			payer, err = types.NewAddressFromString(payerOption.(string))
			if err != nil {
				err = errors.Wrap(err, "invalid payer address")
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
		} else {
			payer = fromAddr
		}

		args, err := abi.ToEncodedValues(payer)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		retValue, retCode, err := n.CallQueryMethod(address.PaymentBrokerAddress, "ls", args, &fromAddr)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if retCode != 0 {
			re.SetError("Non-zero retrurn code executing ls", cmdkit.ErrNormal)
			return
		}

		var channels map[string]*paymentbroker.PaymentChannel
		err = cbor.DecodeInto(retValue[0], &channels)
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
		cmdkit.StringArg("amount", true, false, "filecoin amount of this voucher"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address for which to retrieve channels"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			re.SetError("invalid channel id", cmdkit.ErrNormal)
			return
		}

		amount, ok := types.NewAttoFILFromFILString(req.Arguments[1], 10)
		if !ok {
			re.SetError(ErrInvalidAmount, cmdkit.ErrNormal)
			return
		}

		args, err := abi.ToEncodedValues(channel, amount)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		retValue, retCode, err := n.CallQueryMethod(address.PaymentBrokerAddress, "voucher", args, &fromAddr)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if retCode != 0 {
			re.SetError("Non-zero return code executing voucher", cmdkit.ErrNormal)
			return
		}

		var voucher paymentbroker.PaymentVoucher
		err = cbor.DecodeInto(retValue[0], &voucher)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		// TODO: really sign this thing
		voucher.Signature = fromAddr.Bytes()

		cborVoucher, err := cbor.DumpObject(voucher)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		encoded, err := multibase.Encode(multibase.Base58BTC, cborVoucher)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(encoded) // nolint: errcheck
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
		n := GetNode(env)

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		_, cborVoucher, err := multibase.Decode(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		var voucher paymentbroker.PaymentVoucher
		err = cbor.DecodeInto(cborVoucher, &voucher)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		params, err := abi.ToEncodedValues(voucher.Payer, &voucher.Channel, &voucher.Amount, voucher.Signature)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		// TODO: Sign this message
		msg, err := node.NewMessageWithNextNonce(req.Context, n, fromAddr, address.PaymentBrokerAddress, types.NewAttoFILFromFIL(0), "update", params)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = n.AddNewMessage(req.Context, msg)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		msgCid, err := msg.Cid()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(msgCid) // nolint: errcheck
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
		n := GetNode(env)

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			re.SetError("invalid channel id", cmdkit.ErrNormal)
			return
		}

		params, err := abi.ToEncodedValues(channel)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		// TODO: Sign this message
		msg, err := node.NewMessageWithNextNonce(req.Context, n, fromAddr, address.PaymentBrokerAddress, types.NewAttoFILFromFIL(0), "reclaim", params)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if err := n.AddNewMessage(req.Context, msg); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		msgCid, err := msg.Cid()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(msgCid) // nolint: errcheck
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
		n := GetNode(env)

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		_, cborVoucher, err := multibase.Decode(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		var voucher paymentbroker.PaymentVoucher
		err = cbor.DecodeInto(cborVoucher, &voucher)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		params, err := abi.ToEncodedValues(voucher.Payer, &voucher.Channel, &voucher.Amount, voucher.Signature)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		// TODO: Sign this message
		msg, err := node.NewMessageWithNextNonce(req.Context, n, fromAddr, address.PaymentBrokerAddress, types.NewAttoFILFromFIL(0), "close", params)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = n.AddNewMessage(req.Context, msg)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		msgCid, err := msg.Cid()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(msgCid) // nolint: errcheck
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
		cmdkit.StringArg("amount", true, false, "filecoin amount for the channel"),
		cmdkit.StringArg("eol", true, false, "the block height at which the channel should expire"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address of the channel creator"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			re.SetError("invalid channel id", cmdkit.ErrNormal)
			return
		}

		amount, ok := types.NewAttoFILFromFILString(req.Arguments[1], 10)
		if !ok {
			re.SetError(ErrInvalidAmount, cmdkit.ErrNormal)
			return
		}

		eol, ok := types.NewBlockHeightFromString(req.Arguments[2], 10)
		if !ok {
			re.SetError(ErrInvalidBlockHeight, cmdkit.ErrNormal)
			return
		}

		params, err := abi.ToEncodedValues(channel, eol)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		// TODO: Sign this message
		msg, err := node.NewMessageWithNextNonce(req.Context, n, fromAddr, address.PaymentBrokerAddress, amount, "extend", params)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if err := n.AddNewMessage(req.Context, msg); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		msgCid, err := msg.Cid()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(msgCid) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			fmt.Fprintln(w, c) // nolint: errcheck
			return nil
		}),
	},
}
