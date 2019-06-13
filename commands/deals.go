package commands

import (
	"io"
	"strconv"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"
)

var dealsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage and inspect deals made by or with this node",
	},
	Subcommands: map[string]*cmds.Command{
		"redeem": dealsRedeemCmd,
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
