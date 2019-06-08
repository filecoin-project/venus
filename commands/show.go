package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/types"
)

var showCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get human-readable representations of filecoin objects",
	},
	Subcommands: map[string]*cmds.Command{
		"block": showBlockCmd,
	},
}

var showBlockCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show a filecoin block by its CID",
		ShortDescription: `Prints the miner, parent weight, height,
and nonce of a given block. If JSON encoding is specified with the --enc flag,
all other block properties will be included as well.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "CID of block to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		block, err := GetPorcelainAPI(env).ChainGetBlock(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(block)
	},
	Type: types.Block{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, block *types.Block) error {
			wStr, err := types.FixedStr(uint64(block.ParentWeight))
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(w, `Block Details
Miner:  %s
Weight: %s
Height: %s
Nonce:  %s
`,
				block.Miner,
				wStr,
				strconv.FormatUint(uint64(block.Height), 10),
				strconv.FormatUint(uint64(block.Nonce), 10),
			)
			return err
		}),
	},
}

var showDealCmd = &cmds.Command{
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

		if err := re.Emit(deal); err != nil {
			return err
		}
		return nil
	},
	Type: storagedeal.Deal{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, deal *storagedeal.Deal) error {
			emptyDeal := storagedeal.Deal{}
			if *deal == emptyDeal {
				return fmt.Errorf("deal not found: %s", req.Arguments[0])
			}

			_, err := fmt.Fprintf(w, `Deal details
CID: %s
State: %s
Miner: %s
Duration: %d blocks
Size: %s bytes
Total Price: %s FIL
Payment Vouchers: %s
`,
				deal.Response.ProposalCid,
				deal.Response.State,
				deal.Miner.String(),
				deal.Proposal.Duration,
				deal.Proposal.Size,
				deal.Proposal.TotalPrice,
				vouchersString(deal.Proposal.Payment.Vouchers),
			)
			return err
		}),
	},
}

func vouchersString(vouchers []*types.PaymentVoucher) string {
	if len(vouchers) == 0 {
		return "no payment vouchers"
	}
	sorted := types.SortVouchersByValidAt(vouchers)

	pvStrs := []string{fmt.Sprint("\nIndex\tChannel\tAmount\tValidAt\tEncoded Voucher\n")}
	for i, voucher := range sorted {
		encodedVoucher, err := voucher.Encode()
		if err != nil {
			return err.Error()
		}
		pvStrs = append(pvStrs,
			fmt.Sprintf("%d\t%s\t%s\t%s\t%s\n",
				i, voucher.Channel.String(), voucher.Amount.String(), voucher.ValidAt.String(), encodedVoucher))
	}
	return strings.Join(pvStrs, "\n")
}
