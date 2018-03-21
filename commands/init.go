package commands

import (
	"fmt"
	"io"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/repo"
)

var initCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Initialize a filecoin repo",
	},
	Run: initRun,
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(initTextEncoder),
	},
}

func initRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
	repoDir, ok := req.Options[OptionRepoDir].(string)
	if !ok {
		repoDir = ""
	}

	re.Emit(fmt.Sprintf("initializing filecoin node at %s\n", repoDir)) // nolint: errcheck

	if err := repo.InitFSRepo(repoDir, config.NewDefaultConfig()); err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}
}

func initTextEncoder(req *cmds.Request, w io.Writer, val interface{}) error {
	_, err := fmt.Fprintf(w, val.(string))
	return err
}
