package cmd

import (
	"github.com/filecoin-project/venus/pkg/constants"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

type versionInfo struct {
	// Commit, is the git sha that was used to build this version of venus.
	Version string
}

var versionCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show venus version information",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		return re.Emit(&versionInfo{
			Version: constants.UserVersion(),
		})
	},
	Type: versionInfo{},
}
