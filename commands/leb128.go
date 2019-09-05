package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/filecoin-project/go-leb128"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var leb128Cmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Leb128 cli enc/decode",
		ShortDescription: `Reading and writing leb128.`,
	},
	Subcommands: map[string]*cmds.Command{
		"read":  readLeb128Cmd,
		"write": writeLeb128Cmd,
	},
}

var readLeb128Cmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "read leb128",
		ShortDescription: `Read leb128 to decode`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("text", true, false, `The leb128 encoded text`),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		text := req.Arguments[0]
		val := leb128.ToUInt64([]byte(text))
		return cmds.EmitOnce(res, val)
	},
	Type: uint64(0),
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, info uint64) error {
			_, err := fmt.Fprintln(w, info)
			return err
		}),
	},
}

var writeLeb128Cmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "write leb128",
		ShortDescription: `Write leb128 to encode`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("number", true, false, `The number to encode`),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		num, err := strconv.ParseUint(req.Arguments[0], 10, 64)
		if err != nil {
			return err
		}
		out := leb128.FromUInt64(num)
		return cmds.EmitOnce(res, out)
	},
	Type: []byte{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, info []byte) error {
			_, err := fmt.Fprintln(w, info)
			return err
		}),
	},
}
