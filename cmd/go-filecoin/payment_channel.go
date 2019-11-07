package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var paymentChannelCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Payment channel operations",
	},
	Subcommands: map[string]*cmds.Command{
		"cancel":  cancelCmd,
		"close":   closeCmd,
		"create":  createChannelCmd,
		"extend":  extendCmd,
		"ls":      lsCmd,
		"reclaim": reclaimCmd,
		"redeem":  redeemCmd,
		"voucher": voucherCmd,
	},
}

// CreateChannelResult type returned from CreateChannel
type CreateChannelResult struct {
	Cid     cid.Cid
	GasUsed types.GasUnits
	Preview bool
}

var createChannelCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Create a new payment channel",
		ShortDescription: `Issues a new message to the network to create a payment channeld. Then waits for the
message to be mined to get the channelID.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("target", true, false, "Address of account that will redeem funds"),
		cmdkit.StringArg("amount", true, false, "Amount in FIL for the channel"),
		cmdkit.StringArg("eol", true, false, "The block height at which the channel should expire"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address to send from"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := fromAddrOrDefault(req, env)
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

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		if preview {
			usedGas, err := GetPorcelainAPI(env).MessagePreview(
				req.Context,
				fromAddr,
				address.PaymentBrokerAddress,
				paymentbroker.CreateChannel,
				target, eol,
			)
			if err != nil {
				return err
			}
			return re.Emit(&CreateChannelResult{
				Cid:     cid.Cid{},
				GasUsed: usedGas,
				Preview: true,
			})
		}

		c, _, err := GetPorcelainAPI(env).MessageSend(
			req.Context,
			fromAddr,
			address.PaymentBrokerAddress,
			amount,
			gasPrice,
			gasLimit,
			paymentbroker.CreateChannel,
			target,
			eol,
		)
		if err != nil {
			return err
		}

		return re.Emit(&CreateChannelResult{
			Cid:     c,
			GasUsed: types.NewGasUnits(0),
			Preview: false,
		})
	},
	Type: &CreateChannelResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *CreateChannelResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Cid)
		}),
	},
}

var lsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "List all payment channels for a payer",
		ShortDescription: `Queries the payment broker to find all payment channels where a given account is the payer.`,
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address for which message is sent"),
		cmdkit.StringOption("payer", "Address for which to retrieve channels (defaults to from if omitted)"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		payerOption := req.Options["payer"]
		payerAddr, err := optionalAddr(payerOption)
		if err != nil {
			return err
		}

		channels, err := GetPorcelainAPI(env).PaymentChannelLs(req.Context, fromAddr, payerAddr)
		if err != nil {
			return err
		}

		return re.Emit(channels)
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
		cmdkit.StringArg("channel", true, false, "Channel id of channel from which to create voucher"),
		cmdkit.StringArg("amount", true, false, "Amount in FIL of this voucher"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address for which to retrieve channels"),
		cmdkit.StringOption("validat", "Smallest block height at which target can redeem"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			return fmt.Errorf("invalid channel id")
		}

		amount, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			return ErrInvalidAmount
		}

		validAt, err := optionalBlockHeight(req.Options["validat"])
		if err != nil {
			return err
		}

		voucher, err := GetPorcelainAPI(env).PaymentChannelVoucher(req.Context, fromAddr, channel, amount, validAt, nil)
		if err != nil {
			return err
		}

		v, err := voucher.EncodeBase58()
		if err != nil {
			return err
		}

		return re.Emit(v)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, voucher string) error {
			fmt.Fprintln(w, voucher) // nolint: errcheck
			return nil
		}),
	},
}

// RedeemResult type returned from Redeem
type RedeemResult struct {
	Cid     cid.Cid
	GasUsed types.GasUnits
	Preview bool
}

var redeemCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Redeem a payment voucher against a payment channel",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("voucher", true, false, "Base58 encoded signed voucher"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address of the channel target"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		voucher, err := types.DecodeVoucher(req.Arguments[0])
		if err != nil {
			return err
		}

		result := &ReclaimResult{Preview: preview}

		params := []interface{}{
			voucher.Payer,
			&voucher.Channel,
			voucher.Amount,
			&voucher.ValidAt,
			voucher.Condition,
			[]byte(voucher.Signature),
			[]interface{}{},
		}

		if preview {
			result.GasUsed, err = GetPorcelainAPI(env).MessagePreview(
				req.Context,
				fromAddr,
				address.PaymentBrokerAddress,
				paymentbroker.Redeem,
				params...,
			)
		} else {
			result.Cid, _, err = GetPorcelainAPI(env).MessageSend(
				req.Context,
				fromAddr,
				address.PaymentBrokerAddress,
				types.NewAttoFILFromFIL(0),
				gasPrice,
				gasLimit,
				paymentbroker.Redeem,
				params...,
			)
		}

		if err != nil {
			return err
		}

		return re.Emit(result)
	},
	Type: &RedeemResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *RedeemResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Cid)
		}),
	},
}

// ReclaimResult type returned from Reclaim
type ReclaimResult struct {
	Cid     cid.Cid
	GasUsed types.GasUnits
	Preview bool
}

var reclaimCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Reclaim funds from an expired channel",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("channel", true, false, "Id of channel from which funds are reclaimed"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address of the channel creator"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			return fmt.Errorf("invalid channel id")
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		if preview {
			usedGas, err := GetPorcelainAPI(env).MessagePreview(
				req.Context,
				fromAddr,
				address.PaymentBrokerAddress,
				paymentbroker.Reclaim,
				channel,
			)
			if err != nil {
				return err
			}
			return re.Emit(&ReclaimResult{
				Cid:     cid.Cid{},
				GasUsed: usedGas,
				Preview: true,
			})
		}

		c, _, err := GetPorcelainAPI(env).MessageSend(
			req.Context,
			fromAddr,
			address.PaymentBrokerAddress,
			types.NewAttoFILFromFIL(0),
			gasPrice,
			gasLimit,
			paymentbroker.Reclaim,
			channel,
		)
		if err != nil {
			return err
		}

		return re.Emit(&ReclaimResult{
			Cid:     c,
			GasUsed: types.NewGasUnits(0),
			Preview: false,
		})
	},
	Type: &ReclaimResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *ReclaimResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Cid)
		}),
	},
}

// CloseResult type returned from Close
type CloseResult struct {
	Cid     cid.Cid
	GasUsed types.GasUnits
	Preview bool
}

var closeCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Redeem a payment voucher and close the payment channel",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("voucher", true, false, "Base58 encoded signed voucher"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address of the channel target"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		voucher, err := types.DecodeVoucher(req.Arguments[0])
		if err != nil {
			return err
		}

		result := &CloseResult{Preview: preview}

		params := []interface{}{
			voucher.Payer,
			&voucher.Channel,
			voucher.Amount,
			&voucher.ValidAt,
			voucher.Condition,
			[]byte(voucher.Signature),
			[]interface{}{},
		}

		if preview {
			result.GasUsed, err = GetPorcelainAPI(env).MessagePreview(
				req.Context,
				fromAddr,
				address.PaymentBrokerAddress,
				paymentbroker.Close,
				params...,
			)
		} else {
			result.Cid, _, err = GetPorcelainAPI(env).MessageSend(
				req.Context,
				fromAddr,
				address.PaymentBrokerAddress,
				types.NewAttoFILFromFIL(0),
				gasPrice,
				gasLimit,
				paymentbroker.Close,
				params...,
			)
		}

		if err != nil {
			return err
		}

		return re.Emit(result)
	},
	Type: &CloseResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *CloseResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Cid)
		}),
	},
}

// ExtendResult type returned from Extend
type ExtendResult struct {
	Cid     cid.Cid
	GasUsed types.GasUnits
	Preview bool
}

var extendCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Extend the value and lifetime of a given channel",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("channel", true, false, "Id of channel to extend"),
		cmdkit.StringArg("amount", true, false, "Amount in FIL for the channel"),
		cmdkit.StringArg("eol", true, false, "The block height at which the channel should expire"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "Address of the channel creator"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			return fmt.Errorf("invalid channel id")
		}

		amount, ok := types.NewAttoFILFromFILString(req.Arguments[1])
		if !ok {
			return ErrInvalidAmount
		}

		eol, ok := types.NewBlockHeightFromString(req.Arguments[2], 10)
		if !ok {
			return ErrInvalidBlockHeight
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		if preview {
			usedGas, err := GetPorcelainAPI(env).MessagePreview(
				req.Context,
				fromAddr,
				address.PaymentBrokerAddress,
				paymentbroker.Extend,
				channel, eol,
			)
			if err != nil {
				return err
			}
			return re.Emit(&ExtendResult{
				Cid:     cid.Cid{},
				GasUsed: usedGas,
				Preview: true,
			})
		}

		c, _, err := GetPorcelainAPI(env).MessageSend(
			req.Context,
			fromAddr,
			address.PaymentBrokerAddress,
			amount,
			gasPrice,
			gasLimit,
			paymentbroker.Extend,
			channel, eol,
		)
		if err != nil {
			return err
		}

		return re.Emit(&ExtendResult{
			Cid:     c,
			GasUsed: types.NewGasUnits(0),
			Preview: false,
		})
	},
	Type: &ExtendResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *ExtendResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Cid)
		}),
	},
}

// CancelResult type returned from Cancel
type CancelResult struct {
	Cid     cid.Cid
	GasUsed types.GasUnits
	Preview bool
}

var cancelCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Cancel a payment channel early to recover funds",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("channel", true, false, "id of channel to cancel"),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("from", "address of the channel creator"),
		priceOption,
		limitOption,
		previewOption,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		channel, ok := types.NewChannelIDFromString(req.Arguments[0], 10)
		if !ok {
			return fmt.Errorf("invalid channel id")
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		if preview {
			usedGas, err := GetPorcelainAPI(env).MessagePreview(
				req.Context,
				fromAddr,
				address.PaymentBrokerAddress,
				paymentbroker.Cancel,
				channel,
			)
			if err != nil {
				return err
			}
			return re.Emit(&ExtendResult{
				Cid:     cid.Cid{},
				GasUsed: usedGas,
				Preview: true,
			})
		}

		c, _, err := GetPorcelainAPI(env).MessageSend(
			req.Context,
			fromAddr,
			address.PaymentBrokerAddress,
			types.NewAttoFILFromFIL(0),
			gasPrice,
			gasLimit,
			paymentbroker.Cancel,
			channel,
		)
		if err != nil {
			return err
		}

		return re.Emit(&CancelResult{
			Cid:     c,
			GasUsed: types.NewGasUnits(0),
			Preview: false,
		})
	},
	Type: &CancelResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *CancelResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Cid)
		}),
	},
}
