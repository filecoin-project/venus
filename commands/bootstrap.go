package commands

import (
	"fmt"
	"io"

	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	cmds "gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds"
)

// BootstrapLsResult is the result of the bootstrap listing command.
type BootstrapLsResult struct {
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
		peers, err := GetPorcelainAPI(env).ConfigGet("bootstrap.addresses")
		if err != nil {
			return err
		}

		return re.Emit(&BootstrapLsResult{peers.([]string)})
	},
	Type: &BootstrapLsResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, br *BootstrapLsResult) error {
			_, err := fmt.Fprintln(w, br)
			return err
		}),
	},
}
