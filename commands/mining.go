package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
)

// MiningStatusResult is the type returned when get mining status.
type MiningStatusResult struct {
	Active bool
}

var miningCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage all mining operations for a node",
	},
	Subcommands: map[string]*cmds.Command{
		"once":   miningOnceCmd,
		"start":  miningStartCmd,
		"status": miningStatusCmd,
		"stop":   miningStopCmd,
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
		return re.Emit(&MiningStatusResult{
			Active: isMining,
		})
	},
	Type: &MiningStatusResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MiningStatusResult) error {
			_, err := fmt.Fprintf(w, `Mining Status
Active:  %s
`,
				strconv.FormatBool(res.Active),
			)
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
