package commands

import (
	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var logCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with the daemon event log output.",
		ShortDescription: `
'go-filecoin log' contains utility commands to affect the event logging
output of a running daemon.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"tail": logTailCmd,
	},
}

var logTailCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Read the event log.",
		ShortDescription: `
Outputs event log messages (not other log messages) as they are generated.
`,
	},

	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		r := GetAPI(env).Log().Tail(req.Context)
		re.Emit(r) // nolint: errcheck
	},
}
