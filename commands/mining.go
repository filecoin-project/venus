package commands

import (
	"fmt"
	"io"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var miningCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage all mining operations for a node",
	},
	Subcommands: map[string]*cmds.Command{
		"once":  miningOnceCmd,
		"start": miningStartCmd,
		"stop":  miningStopCmd,
	},
}

var miningOnceCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		blk, err := GetAPI(env).Mining().Once(req.Context)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		re.Emit(blk.Cid()) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			fmt.Fprintln(w, c) // nolint: errcheck
			return nil
		}),
	},
}

var miningStartCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		if err := GetAPI(env).Mining().Start(req.Context); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		re.Emit("Started mining") // nolint: errcheck
	},
	Type:     "",
	Encoders: stringEncoderMap,
}

var miningStopCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		if err := GetAPI(env).Mining().Stop(req.Context); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		re.Emit("Stopped mining") // nolint: errcheck
	},
	Encoders: stringEncoderMap,
}

var stringEncoderMap = cmds.EncoderMap{
	cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, t string) error {
		fmt.Fprintln(w, t) // nolint: errcheck
		return nil
	}),
}
