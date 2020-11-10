package commands

import (
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/venus/build/flags"
)

type versionInfo struct {
	// Commit, is the git sha that was used to build this version of go-filecoin.
	Commit string
}

var versionCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show go-filecoin version information",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		return re.Emit(&versionInfo{
			Commit: flags.GitCommit,
		})
	},
	Type: versionInfo{},
}
