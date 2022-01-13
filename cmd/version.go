package cmd

import (
	"fmt"

	"github.com/filecoin-project/venus/build/flags"
	"github.com/filecoin-project/venus/pkg/constants"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

type versionInfo struct {
	// Commit, is the git sha that was used to build this version of venus.
	Commit string
}

var versionCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show venus version information",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		return re.Emit(&versionInfo{
			Commit: fmt.Sprintf("%s %s", constants.BuildVersion, flags.GitCommit),
		})
	},
	Type: versionInfo{},
}
