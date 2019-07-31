package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/address"
)

// MiningStatusResult is the type returned when get mining status.
type MiningStatusResult struct {
	Active        bool                      `json:"active"`
	Miner         address.Address           `json:"minerAddress"`
	ProvingPeriod *MinerProvingPeriodResult `json:"provingPeriod,omitempty"`
}

var miningCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage all mining operations for a node",
	},
	Subcommands: map[string]*cmds.Command{
		"address": miningAddrCmd,
		"once":    miningOnceCmd,
		"start":   miningStartCmd,
		"status":  miningStatusCmd,
		"stop":    miningStopCmd,
	},
}

var miningAddrCmd = &cmds.Command{
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

var miningStartCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if err := GetBlockAPI(env).MiningStart(req.Context); err != nil {
			return err
		}
		return re.Emit("Started mining")
	},
	Type:     "",
	Encoders: stringEncoderMap,
}

var miningStatusCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		isMining := GetBlockAPI(env).MiningIsActive()

		if !isMining {
			return re.Emit(&MiningStatusResult{
				Active: isMining,
			})
		}
		// Get the Miner Address
		minerAddress, err := GetBlockAPI(env).MinerAddress()
		if err != nil {
			return err
		}

		mpp, err := GetMinerProvingPeriod(req.Context, minerAddress, GetPorcelainAPI(env))
		if err != nil {
			return err
		}

		return re.Emit(&MiningStatusResult{
			Active:        isMining,
			Miner:         minerAddress,
			ProvingPeriod: mpp,
		})
	},
	Type: &MiningStatusResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MiningStatusResult) error {
			_, err := fmt.Fprintf(w, `Mining Status
Active:  %s
Address: %s
`, strconv.FormatBool(res.Active), res.Miner.String())
			if err != nil {
				return err
			}

			if res.Active {
				var pSet []string
				for p := range res.ProvingPeriod.Set {
					pSet = append(pSet, p)
				}
				_, err = fmt.Fprintf(w, `Proving Period Start: %s
Proving Period End: %s
Proving Period Set: %s
`,
					res.ProvingPeriod.Start.String(),
					res.ProvingPeriod.End.String(),
					pSet,
				)
			}
			return err
		}),
	},
}

var miningStopCmd = &cmds.Command{
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
