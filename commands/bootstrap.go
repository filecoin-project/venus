package commands

import (
	"fmt"
	"io"

	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
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
		n := GetNode(env)
		peers := n.Repo.Config().Bootstrap.Addresses

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
