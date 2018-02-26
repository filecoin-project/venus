package commands

import (
	"fmt"
	"io"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/flags"
)

var versionCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show go-filecoin version information",
	},
	Run: versionRun,
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

func versionRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
	re.Emit(&versionOutput{Commit: flags.Commit}) // nolint: errcheck
}
