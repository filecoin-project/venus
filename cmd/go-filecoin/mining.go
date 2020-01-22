package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

var miningCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage all mining operations for a node",
	},
	Subcommands: map[string]*cmds.Command{
		"address":   miningAddrCmd,
		"once":      miningOnceCmd,
		"start":     miningStartCmd,
		"status":    miningStatusCmd,
		"stop":      miningStopCmd,
		"setup":     miningSetupCmd,
		"add-piece": miningAddPieceCmd,
	},
}

var miningAddrCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Retrieve address of miner actor associated with this node",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		minerAddress, err := GetBlockAPI(env).MinerAddress()
		if err != nil {
			return err
		}
		return re.Emit(minerAddress.String())
	},
	Type: address.Address{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a address.Address) error {
			fmt.Fprintln(w, a.String()) // nolint: errcheck
			return nil
		}),
	},
}

var miningOnceCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Mine a single block",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		blk, err := GetBlockAPI(env).MiningOnce(req.Context)
		if err != nil {
			return err
		}
		return re.Emit(blk.Cid())
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c cid.Cid) error {
			fmt.Fprintln(w, c) // nolint: errcheck
			return nil
		}),
	},
}

var miningSetupCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Prepare node to receive storage deals without starting the mining scheduler",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if err := GetBlockAPI(env).MiningSetup(req.Context); err != nil {
			return err
		}
		return re.Emit("mining ready")
	},
	Type:     "",
	Encoders: stringEncoderMap,
}

var miningStartCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Start mining blocks and other mining related operations",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if err := GetBlockAPI(env).MiningStart(req.Context); err != nil {
			return err
		}
		return re.Emit("Started mining")
	},
	Type:     "",
	Encoders: stringEncoderMap,
}

// MiningStatusResult is the type returned when get mining status.
type MiningStatusResult struct {
	Active        bool                         `json:"active"`
	Miner         address.Address              `json:"minerAddress"`
	Owner         address.Address              `json:"owner"`
	Collateral    types.AttoFIL                `json:"collateral"`
	ProvingPeriod porcelain.MinerProvingWindow `json:"provingPeriod,omitempty"`
	Power         porcelain.MinerPower         `json:"minerPower"`
}

var miningStatusCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Report on mining status",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		isMining := GetBlockAPI(env).MiningIsActive()

		// Get the Miner Address
		minerAddress, err := GetBlockAPI(env).MinerAddress()
		if err != nil {
			return err
		}

		mpp, err := GetPorcelainAPI(env).MinerGetProvingWindow(req.Context, minerAddress)
		if err != nil {
			return err
		}

		owner, err := GetPorcelainAPI(env).MinerGetOwnerAddress(req.Context, minerAddress)
		if err != nil {
			return err
		}

		collateral, err := GetPorcelainAPI(env).MinerGetCollateral(req.Context, minerAddress)
		if err != nil {
			return err
		}

		power, err := GetPorcelainAPI(env).MinerGetPower(req.Context, minerAddress)
		if err != nil {
			return err
		}

		return re.Emit(&MiningStatusResult{
			Active:        isMining,
			Miner:         minerAddress,
			Owner:         owner,
			Collateral:    collateral,
			Power:         power,
			ProvingPeriod: mpp,
		})
	},
	Type: &MiningStatusResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MiningStatusResult) error {
			var pSet []string
			for p := range res.ProvingPeriod.ProvingSet {
				pSet = append(pSet, p)
			}
			_, err := fmt.Fprintf(w, `Mining Status
Active:     %s
Address:    %s
Owner:      %s
Collateral: %s
Power:      %s / %s

Proving Period
Start:         %s
End:           %s
Proving Set:   %s

`, strconv.FormatBool(res.Active),
				res.Miner.String(),
				res.Owner.String(),
				res.Collateral.String(),
				res.Power.Power.String(), res.Power.Total.String(),
				res.ProvingPeriod.Start.String(),
				res.ProvingPeriod.End.String(),
				pSet)

			return err
		}),
	},
}

var miningStopCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Stop block mining",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		GetBlockAPI(env).MiningStop(req.Context)
		return re.Emit("Stopped mining")
	},
	Encoders: stringEncoderMap,
}

var stringEncoderMap = cmds.EncoderMap{
	cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, t string) error {
		fmt.Fprintln(w, t) // nolint: errcheck
		return nil
	}),
}

// MiningAddPieceResult is a wrapper around the uint64 sectorID
type MiningAddPieceResult struct {
	SectorID uint64
}

var miningAddPieceCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Add data directly to a staged sector",
		ShortDescription: `
Adds a piece (a local file) to a staged sector.  This is used
to add data outside of a deal.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("file", true, false, "Path of file to add").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		panic("TODO: rework this command to create a self-deal")
	},
	Type: MiningAddPieceResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, result MiningAddPieceResult) error {
			fmt.Fprintf(w, "piece staged in sector %d\n", result.SectorID) // nolint: errcheck
			return nil
		}),
	},
}
