package commands

import (
	"strconv"

	"github.com/filecoin-project/go-leb128"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var leb128Cmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Leb128 cli encode/decode",
		ShortDescription: `Decode and encode leb128 text/uint64.`,
	},
	Subcommands: map[string]*cmds.Command{
		"decode": decodeLeb128Cmd,
		"encode": encodeLeb128Cmd,
	},
}

var decodeLeb128Cmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "decode leb128",
		ShortDescription: `Decode leb128 text`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("text", true, false, `The leb128 encoded text`),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		text := req.Arguments[0]
		val := leb128.ToUInt64([]byte(text))
		return cmds.EmitOnce(res, val)
	},
	Type: uint64(0),
}

var encodeLeb128Cmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "encode leb128",
		ShortDescription: `Encode leb128 uint64`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("number", true, false, `The number to encode`),
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
}
