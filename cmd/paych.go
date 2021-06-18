package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/paychmgr"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/paych"
	"github.com/filecoin-project/venus/pkg/types"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"golang.org/x/xerrors"
)

var paychCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manage payment channels",
	},
	Subcommands: map[string]*cmds.Command{
		"add-funds":         addFundsCmd,
		"list":              listCmd,
		"voucher":           voucherCmd,
		"settle":            settleCmd,
		"status":            statusCmd,
		"status-by-from-to": sbftCmd,
		"collect":           collectCmd,
	},
}

var addFundsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add funds to the payment channel between fromAddress and toAddress. Creates the payment channel if it doesn't already exist.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("from_addr", true, false, "From Address is the payment channel sender"),
		cmds.StringArg("to_addr", true, false, "To Address is the payment channel recipient"),
		cmds.StringArg("amount", true, false, "Amount is the deposits funds in the payment channel"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		toAddr, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}
		amt, err := types.ParseFIL(req.Arguments[2])
		if err != nil {
			return err
		}
		chanInfo, err := env.(*node.Env).PaychAPI.PaychGet(req.Context, fromAddr, toAddr, big.NewFromGo(amt.Int))
		if err != nil {
			return err
		}
		chAddr, err := env.(*node.Env).PaychAPI.PaychGetWaitReady(req.Context, chanInfo.WaitSentinel)
		if err != nil {
			return err
		}
		if err := re.Emit(chAddr); err != nil {
			return err
		}
		return nil
	},
}

var listCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List all locally registered payment channels",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addrs, err := env.(*node.Env).PaychAPI.PaychList(req.Context)
		if err != nil {
			return err
		}
		if err := re.Emit(addrs); err != nil {
			return err
		}
		return nil
	},
}

var voucherCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with payment channel vouchers",
	},
	Subcommands: map[string]*cmds.Command{
		"create":         voucherCreateCmd,
		"check":          voucherCheckCmd,
		"add":            voucherAddCmd,
		"list":           voucherListCmd,
		"best-spendable": voucherBestSpendableCmd,
		"submit":         voucherSubmitCmd,
	},
}

var settleCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Settle a payment channel",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("channel_addr", true, false, "The given payment channel address"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		chanAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		mcid, err := env.(*node.Env).PaychAPI.PaychSettle(req.Context, chanAddr)
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
		mwait, err := env.(*node.Env).ChainAPI.StateWaitMsg(req.Context, mcid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}
		if mwait.Receipt.ExitCode != 0 {
			return xerrors.Errorf("settle message execution failed (exit code %d)", mwait.Receipt.ExitCode)
		}
		if err := re.Emit(fmt.Sprintf("Settled channel %s", chanAddr)); err != nil {
			return err
		}
		return nil
	},
}
var statusCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show the status of an outbound payment channel",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("channel_addr", true, false, "The given payment channel address"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		chanAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		av, err := env.(*node.Env).PaychAPI.PaychAvailableFunds(req.Context, chanAddr)
		if err != nil {
			return err
		}
		//re.Emit(av)
		w := bytes.NewBuffer(nil)
		paychStatus(w, av)
		if err := re.Emit(w); err != nil {
			return err
		}
		return nil
	},
}
var sbftCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show the status of an active outbound payment channel by from/to addresses",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("from_addr", true, false, "Gets a channel accessor for a given from / to pair"),
		cmds.StringArg("to_addr", true, false, "Gets a channel accessor for a given from / to pair"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fromAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		toAddr, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}
		av, err := env.(*node.Env).PaychAPI.PaychAvailableFundsByFromTo(req.Context, fromAddr, toAddr)
		if err != nil {
			return err
		}
		w := bytes.NewBuffer(nil)
		paychStatus(w, av)
		if err := re.Emit(w); err != nil {
			return err
		}
		return nil
	},
}
var collectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create a signed payment channel voucher",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("channel_addr", true, false, "The given payment channel address"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		chanAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		mcid, err := env.(*node.Env).PaychAPI.PaychCollect(req.Context, chanAddr)
		if err != nil {
			return err
		}
		mwait, err := env.(*node.Env).ChainAPI.StateWaitMsg(req.Context, mcid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}
		if mwait.Receipt.ExitCode != 0 {
			return xerrors.Errorf("collect message execution failed (exit code %d)", mwait.Receipt.ExitCode)
		}

		if err := re.Emit(fmt.Sprintf("Collected funds for channel %s", chanAddr)); err != nil {
			return err
		}
		return nil
	},
}

var voucherCreateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create a signed payment channel voucher",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("channel_addr", true, false, "The given payment channel address"),
		cmds.StringArg("amount", true, false, "The value that will be used to create the voucher"),
		cmds.StringArg("lane", true, false, "Specify payment channel lane to use"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		chanAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		amtFil, err := types.ParseFIL(req.Arguments[1])
		if err != nil {
			return err
		}
		lane, err := strconv.ParseUint(req.Arguments[2], 10, 64)
		if err != nil {
			return err
		}
		res, err := env.(*node.Env).PaychAPI.PaychVoucherCreate(req.Context, chanAddr, big.NewFromGo(amtFil.Int), lane)
		if err != nil {
			return err
		}
		if res.Voucher == nil {
			return xerrors.Errorf("Could not create voucher: insufficient funds in channel, shortfall: %d", res.Shortfall)
		}
		enc, err := encodedString(res.Voucher)
		if err != nil {
			return err
		}

		if err := re.Emit(enc); err != nil {
			return err
		}
		return nil
	},
}

var voucherCheckCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Check validity of payment channel voucher",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("channel_addr", true, false, "The given payment channel address"),
		cmds.StringArg("voucher", true, false, "The voucher in the payment channel"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		chanAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		voucher, err := paych.DecodeSignedVoucher(req.Arguments[1])
		if err != nil {
			return err
		}
		err = env.(*node.Env).PaychAPI.PaychVoucherCheckValid(req.Context, chanAddr, voucher)
		if err != nil {
			return err
		}
		if err := re.Emit("voucher is valid"); err != nil {
			return err
		}
		return nil
	},
}

var voucherAddCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add payment channel voucher to local datastore",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("channel_addr", true, false, "The given payment channel address"),
		cmds.StringArg("voucher", true, false, "The voucher in the payment channel"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		chanAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		voucher, err := paych.DecodeSignedVoucher(req.Arguments[1])
		if err != nil {
			return err
		}
		_, err = env.(*node.Env).PaychAPI.PaychVoucherAdd(req.Context, chanAddr, voucher, nil, big.NewInt(0))
		if err != nil {
			return err
		}
		if err := re.Emit("add voucher successfully"); err != nil {
			return err
		}
		return nil
	},
}

var voucherListCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List stored vouchers for a given payment channel",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("channel_addr", true, false, "The given payment channel address"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		chanAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		vs, err := env.(*node.Env).PaychAPI.PaychVoucherList(req.Context, chanAddr)
		if err != nil {
			return err
		}
		buff := bytes.NewBuffer(nil)
		for _, v := range sortVouchers(vs) {
			str, err := encodedString(v)
			if err != nil {
				return err
			}
			fmt.Fprintf(buff, "Lane %d, Nonce %d: %s, voucher: %s\n", v.Lane, v.Nonce, v.Amount.String(), str)
		}

		if err := re.Emit(buff); err != nil {
			return err
		}
		return nil
	},
}

var voucherBestSpendableCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print vouchers with highest value that is currently spendable for each lane",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("channel_addr", true, false, "The given payment channel address"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		chanAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		vouchersByLane, err := paychmgr.BestSpendableByLane(req.Context, env.(*node.Env).PaychAPI, chanAddr)
		if err != nil {
			return err
		}

		var vouchers []*paych.SignedVoucher
		for _, vchr := range vouchersByLane {
			vouchers = append(vouchers, vchr)
		}
		buff := bytes.NewBuffer(nil)
		for _, v := range sortVouchers(vouchers) {
			str, err := encodedString(v)
			if err != nil {
				return err
			}
			fmt.Fprintf(buff, "Lane %d, Nonce %d: %s, voucher: %s\n", v.Lane, v.Nonce, v.Amount.String(), str)
		}
		if err := re.Emit(buff); err != nil {
			return err
		}
		return nil
	},
}
var voucherSubmitCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Submit voucher to chain to update payment channel state",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("channel_addr", true, false, "The given payment channel address"),
		cmds.StringArg("voucher", true, false, "The voucher in the payment channel"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		chanAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		voucher, err := paych.DecodeSignedVoucher(req.Arguments[1])
		if err != nil {
			return err
		}
		mcid, err := env.(*node.Env).PaychAPI.PaychVoucherSubmit(req.Context, chanAddr, voucher, nil, nil)
		if err != nil {
			return err
		}
		mwait, err := env.(*node.Env).ChainAPI.StateWaitMsg(req.Context, mcid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}
		if mwait.Receipt.ExitCode != 0 {
			return xerrors.Errorf("message execution failed (exit code %d)", mwait.Receipt.ExitCode)
		}
		if err := re.Emit("channel updated successfully"); err != nil {
			return err
		}
		return nil
	},
}

func encodedString(sv *paych.SignedVoucher) (string, error) {
	buf := new(bytes.Buffer)
	if err := sv.MarshalCBOR(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}
func sortVouchers(vouchers []*paych.SignedVoucher) []*paych.SignedVoucher {
	sort.Slice(vouchers, func(i, j int) bool {
		if vouchers[i].Lane == vouchers[j].Lane {
			return vouchers[i].Nonce < vouchers[j].Nonce
		}
		return vouchers[i].Lane < vouchers[j].Lane
	})
	return vouchers
}
func paychStatus(writer io.Writer, avail *apitypes.ChannelAvailableFunds) {
	if avail.Channel == nil {
		if avail.PendingWaitSentinel != nil {
			fmt.Fprint(writer, "Creating channel\n")
			fmt.Fprintf(writer, "  From:          %s\n", avail.From)
			fmt.Fprintf(writer, "  To:            %s\n", avail.To)
			fmt.Fprintf(writer, "  Pending Amt:   %d\n", avail.PendingAmt)
			fmt.Fprintf(writer, "  Wait Sentinel: %s\n", avail.PendingWaitSentinel)
			return
		}
		fmt.Fprint(writer, "Channel does not exist\n")
		fmt.Fprintf(writer, "  From: %s\n", avail.From)
		fmt.Fprintf(writer, "  To:   %s\n", avail.To)
		return
	}

	if avail.PendingWaitSentinel != nil {
		fmt.Fprint(writer, "Adding Funds to channel\n")
	} else {
		fmt.Fprint(writer, "Channel exists\n")
	}
	nameValues := [][]string{
		{"Channel", avail.Channel.String()},
		{"From", avail.From.String()},
		{"To", avail.To.String()},
		{"Confirmed Amt", fmt.Sprintf("%d", avail.ConfirmedAmt)},
		{"Pending Amt", fmt.Sprintf("%d", avail.PendingAmt)},
		{"Queued Amt", fmt.Sprintf("%d", avail.QueuedAmt)},
		{"Voucher Redeemed Amt", fmt.Sprintf("%d", avail.VoucherReedeemedAmt)},
	}
	if avail.PendingWaitSentinel != nil {
		nameValues = append(nameValues, []string{
			"Add Funds Wait Sentinel",
			avail.PendingWaitSentinel.String(),
		})
	}
	fmt.Fprint(writer, formatNameValues(nameValues))
}

func formatNameValues(nameValues [][]string) string {
	maxLen := 0
	for _, nv := range nameValues {
		if len(nv[0]) > maxLen {
			maxLen = len(nv[0])
		}
	}
	out := make([]string, len(nameValues))
	for i, nv := range nameValues {
		namePad := strings.Repeat(" ", maxLen-len(nv[0]))
		out[i] = "  " + nv[0] + ": " + namePad + nv[1]
	}
	return strings.Join(out, "\n") + "\n"
}
