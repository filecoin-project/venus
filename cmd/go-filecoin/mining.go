package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var miningCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage all mining operations for a node",
	},
	Subcommands: map[string]*cmds.Command{
		"address":       miningAddrCmd,
		"once":          miningOnceCmd,
		"start":         miningStartCmd,
		"status":        miningStatusCmd,
		"stop":          miningStopCmd,
		"setup":         miningSetupCmd,
		"pledge-sector": miningPledgeSectorCmd,
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
	Miner  address.Address `json:"minerAddress"`
	Active bool            `json:"active"`
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

		return re.Emit(&MiningStatusResult{
			Miner:  minerAddress,
			Active: isMining,
		})
	},
	Type: &MiningStatusResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MiningStatusResult) error {
			_, err := fmt.Fprintf(w, `Mining Status
Active:     %s
Address:    %s
`, strconv.FormatBool(res.Active), res.Miner)

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

var miningPledgeSectorCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Pledge an empty sector immediately",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		err := GetStorageAPI(env).PledgeSector(req.Context)
		if err != nil {
			return err
		}
		return re.Emit("Sector pledged")
	},
	Type:     "",
	Encoders: stringEncoderMap,
}
