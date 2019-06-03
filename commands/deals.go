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
)

var dealsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage and inspect deals made by or with this node",
	},
	Subcommands: map[string]*cmds.Command{
		"list":   dealsListCmd,
		"redeem": dealsRedeemCmd,
	},
}

type dealsListResult struct {
	Miner       address.Address   `json:"minerAddress"`
	PieceCid    cid.Cid           `json:"pieceCid"`
	ProposalCid cid.Cid           `json:"proposalCid"`
	State       storagedeal.State `json:"state"`
}

var dealsListCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List all deals",
		ShortDescription: `
Lists all recorded deals made by or with this node. This may include pending
deals, active deals, finished deals and cancelled deals.
`,
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		dealsCh, err := GetPorcelainAPI(env).DealsLs(req.Context)
		if err != nil {
			return err
		}

		for deal := range dealsCh {
			if deal.Err != nil {
				return deal.Err
			}
			out := &dealsListResult{
				Miner:       deal.Deal.Miner,
				PieceCid:    deal.Deal.Proposal.PieceRef,
				ProposalCid: deal.Deal.Response.ProposalCid,
				State:       deal.Deal.Response.State,
			}
			if err = re.Emit(out); err != nil {
				return err
			}
		}

		return nil
	},
	Type: dealsListResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *dealsListResult) error {
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
		fromAddr, err := optionalAddr(req.Options["from"])
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
