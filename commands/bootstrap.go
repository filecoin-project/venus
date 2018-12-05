package commands

import (
	"fmt"
	"io"

	cmds "gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

type bootstrapResult struct {
	Peers []string
}

var bootstrapCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with bootstrap addresses",
	},
	Subcommands: map[string]*cmds.Command{
		"ls": bootstrapLsCmd,
	},
}

var bootstrapLsCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		peers, err := GetAPI(env).Bootstrap().Ls(req.Context)
		if err != nil {
			return err
		}

		re.Emit(&bootstrapResult{peers}) // nolint: errcheck
		return nil
	},
	Type: &bootstrapResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, br *bootstrapResult) error {
			_, err := fmt.Fprintln(w, br.Peers)
			return err
		}),
	},
}
