package commands

import (
	"time"

	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"

	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
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
		"tail":     logTailCmd,
		"streamto": logStreamToCmd,
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

var logStreamToCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Stream the json encoded log events to a multiaddress.",
		ShortDescription: `
Sends all json-encoded log messages to the multiaddr.
This command will run until its killed.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("addr", true, false, "multiaddress logs will stream to"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		maddr, err := ma.NewMultiaddr(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		for {
			if req.Context.Err() != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
			err := GetAPI(env).Log().StreamTo(req.Context, maddr)
			log.Warningf("StreamTo closed, re-starting... %v", err)
			time.Sleep(1 * time.Second)
		}
	},
}
