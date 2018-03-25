package commands

import (
	"fmt"
	"io"

	config "github.com/filecoin-project/go-filecoin/config"

	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
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
		"list": bootstrapListCmd,
	},
}

var bootstrapListCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		// TODO load from config file once implemented
		cfg := config.NewDefaultConfig()

		peers := cfg.Bootstrap.Addresses

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
