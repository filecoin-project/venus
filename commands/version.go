package commands

import (
	"fmt"
	"io"

	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/flags"
)

var versionCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show go-filecoin version information",
	},
	Run:  versionRun,
	Type: versionOutput{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, vo *versionOutput) error {
			_, err := fmt.Fprintf(w, "commit: %s\n", vo.Commit)
			return err
		}),
	},
}

type versionOutput struct {
	Commit string
}

func versionRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	re.Emit(&versionOutput{Commit: flags.Commit}) // nolint: errcheck

	return nil
}
