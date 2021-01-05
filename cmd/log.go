package cmd

import (
	"fmt"
	"strings"

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log/v2"
)

var logCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with the daemon subsystems log output.",
		ShortDescription: `
'venus log' contains utility commands to affect the subsystems logging
output of a running daemon.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"set-level": logLevelCmd,
		"list":      logLsCmd,
		"tail":      logTailCmd,
	},
}

var logTailCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Read subsystems log output.",
		ShortDescription: `
Outputs subsystems log output as it is generated.
`,
	},

	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		r := logging.NewPipeReader()
		go func() {
			defer r.Close() // nolint: errcheck
			<-req.Context.Done()
		}()

		return re.Emit(r)
	},
}

var logLevelCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Set the logging level.",
		ShortDescription: `Set the log level for logging systems:

   The system flag can be specified multiple times.

   eg) log set-level --system chain --system pubsub debug

   Available Levels:
   debug
   info
   warn
   error
   fatal
   panic
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("set-level", true, false, `The log level, with 'debug' the most verbose and 'panic' the least verbose.
			One of: debug, info, warning, error, fatal, panic.
		`),
	},

	Options: []cmds.Option{
		cmds.StringsOption("system", "The system logging identifier"),
	},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		level := strings.ToLower(req.Arguments[0])

		var s string
		if system, ok := req.Options["system"].([]string); ok {
			for _, v := range system {
				if err := logging.SetLogLevel(v, level); err != nil {
					return err
				}
			}
			s = fmt.Sprintf("Set log level of '%s' to '%s'", strings.Join(system, ","), level)
		} else {
			lvl, err := logging.LevelFromString(level)
			if err != nil {
				return err
			}
			logging.SetAllLoggers(lvl)
			s = fmt.Sprintf("Set log level of all subsystems to: %s", level)
		}

		return cmds.EmitOnce(res, s)
	},
	Type: string(""),
}

var logLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List the logging subsystems.",
		ShortDescription: `
'venus log list' is a utility command used to list the logging
subsystems of a running daemon.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		return cmds.EmitOnce(res, logging.GetSubsystems())
	},
	Type: []string{},
}
