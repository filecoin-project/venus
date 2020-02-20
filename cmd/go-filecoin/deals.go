package commands

import (
	"encoding/json"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
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
		"list": dealsListCmd,
		"show": dealsShowCmd,
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
		//minerAddress, _ := GetPorcelainAPI(env).ConfigGet("mining.minerAddress")
		panic("implement me in terms of the storage market module")
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

// DealsShowResult contains Deal output with Payment Vouchers.
type DealsShowResult struct {
	DealCID    cid.Cid            `json:"deal_cid"`
	State      market.State       `json:"state"`
	Miner      *address.Address   `json:"miner_address"`
	Duration   uint64             `json:"duration_blocks"`
	Size       *types.BytesAmount `json:"deal_size"`
	TotalPrice *types.AttoFIL     `json:"total_price"`
}

var dealsShowCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show deal details for CID <cid>",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "CID of deal to query"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		panic("implement me in terms of the storage market module")
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
