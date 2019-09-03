package commands

import (
	"encoding/json"
	"io"
	"strconv"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/types"
)

const (
	clientOnly = "client"
	minerOnly  = "miner"
)

var dealsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage and inspect deals made by or with this node",
	},
	Subcommands: map[string]*cmds.Command{
		"list":   dealsListCmd,
		"redeem": dealsRedeemCmd,
		"show":   dealsShowCmd,
	},
}

// DealsListResult represents the subset of deal data returned by deals list
type DealsListResult struct {
	Miner       address.Address `json:"minerAddress"`
	PieceCid    cid.Cid         `json:"pieceCid"`
	ProposalCid cid.Cid         `json:"proposalCid"`
	State       string          `json:"state"`
}

var dealsListCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List all deals",
		ShortDescription: `
Lists all recorded deals made by or with this node. This may include pending
deals, active deals, finished deals and cancelled deals.
`,
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(clientOnly, "c", "only return deals made as a client"),
		cmdkit.BoolOption(minerOnly, "m", "only return deals made as a miner"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddress, _ := GetPorcelainAPI(env).ConfigGet("mining.minerAddress")
		dealsCh, err := GetPorcelainAPI(env).DealsLs(req.Context)
		if err != nil {
			return err
		}

		filterForMiner, _ := req.Options[minerOnly].(bool)
		filterForClient, _ := req.Options[clientOnly].(bool)

		for deal := range dealsCh {
			if deal.Err != nil {
				return deal.Err
			}
			if filterForMiner && deal.Deal.Miner != minerAddress {
				continue
			}
			if filterForClient && deal.Deal.Miner == minerAddress {
				continue
			}
			out := &DealsListResult{
				Miner:       deal.Deal.Miner,
				PieceCid:    deal.Deal.Proposal.PieceRef,
				ProposalCid: deal.Deal.Response.ProposalCid,
				State:       deal.Deal.Response.State.String(),
			}
			if err = re.Emit(out); err != nil {
				return err
			}
		}

		return nil
	},
	Type: DealsListResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *DealsListResult) error {
			encoder := json.NewEncoder(w)
			encoder.SetIndent("", "\t")
			return encoder.Encode(res)
		}),
	},
}

var dealsRedeemCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Redeem vouchers for a deal",
		ShortDescription: `
Redeem vouchers for FIL on the storage deal specified with the given deal CID.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("dealid", true, false, "CID of the deal to redeem"),
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

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		dealCid, err := cid.Parse(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid cid "+req.Arguments[0])
		}

		result := &RedeemResult{Preview: preview}

		if preview {
			result.GasUsed, err = GetPorcelainAPI(env).DealRedeemPreview(
				req.Context,
				fromAddr,
				dealCid,
			)
		} else {
			result.Cid, err = GetPorcelainAPI(env).DealRedeem(
				req.Context,
				fromAddr,
				dealCid,
				gasPrice,
				gasLimit,
			)
		}

		if err != nil {
			return err
		}

		return re.Emit(result)
	},
	Type: RedeemResult{},
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

// DealsShowResult contains Deal output with Payment Vouchers.
type DealsShowResult struct {
	DealCID         cid.Cid                `json:"deal_cid"`
	State           storagedeal.State      `json:"state"`
	Miner           *address.Address       `json:"miner_address"`
	Duration        uint64                 `json:"duration_blocks"`
	Size            *types.BytesAmount     `json:"deal_size"`
	TotalPrice      *types.AttoFIL         `json:"total_price"`
	PaymentVouchers []*PaymenVoucherResult `json:"payment_vouchers"`
}

// PaymenVoucherResult is selected PaymentVoucher fields,
// the Index of each when sorted by increasing ValidAt,
// and the string encoded version of the PaymentVoucher
type PaymenVoucherResult struct {
	Index     uint64             `json:"index"`
	Amount    *types.AttoFIL     `json:"amount"`
	Channel   *types.ChannelID   `json:"channel_id"`
	Condition *types.Predicate   `json:"condition"`
	Payer     *address.Address   `json:"payer"`
	ValidAt   *types.BlockHeight `json:"valid_at_block"`
	EncodedAs string             `json:"encoded_as"`
}

var dealsShowCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show deal details for CID <cid>",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "CID of deal to query"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		propcid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		deal, err := GetPorcelainAPI(env).DealGet(req.Context, propcid)
		if err != nil {
			return err
		}

		vouchers, err := paymentVouchersResult(deal.Proposal.Payment.Vouchers)
		if err != nil {
			return err
		}

		out := &DealsShowResult{
			DealCID:         deal.Response.ProposalCid,
			State:           deal.Response.State,
			Miner:           &deal.Miner,
			Duration:        deal.Proposal.Duration,
			Size:            deal.Proposal.Size,
			TotalPrice:      &deal.Proposal.TotalPrice,
			PaymentVouchers: vouchers,
		}

		if err := re.Emit(out); err != nil {
			return err
		}
		return nil
	},
	Type: DealsShowResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, dealResult *DealsShowResult) error {
			encoder := json.NewEncoder(w)
			encoder.SetIndent("", "\t")
			return encoder.Encode(dealResult)
		}),
	},
}

func paymentVouchersResult(vouchers []*types.PaymentVoucher) (pvres []*PaymenVoucherResult, err error) {
	if len(vouchers) == 0 {
		return pvres, nil
	}
	sorted := types.SortVouchersByValidAt(vouchers)

	for i, voucher := range sorted {
		encodedVoucher, err := voucher.Encode()
		if err != nil {
			return pvres, err
		}
		pvres = append(pvres, &PaymenVoucherResult{
			Index:     uint64(i),
			Amount:    &voucher.Amount,
			Channel:   &voucher.Channel,
			Condition: voucher.Condition,
			Payer:     &voucher.Payer,
			ValidAt:   &voucher.ValidAt,
			EncodedAs: encodedVoucher,
		})
	}
	return pvres, nil
}
