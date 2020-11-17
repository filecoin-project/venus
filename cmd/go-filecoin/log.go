package commands

import (
	"fmt"
	"strings"

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log/v2"
)

var loglogger = logging.Logger("commands/log")

var logCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with the daemon subsystems log output.",
		ShortDescription: `
'venus log' contains utility commands to affect the subsystems logging
output of a running daemon.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"level": logLevelCmd,
		"ls":    logLsCmd,
		"tail":  logTailCmd,
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
		Tagline: "Change the logging level.",
		ShortDescription: `
Change the verbosity of one or all subsystems log output. This does not affect
the event log.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("level", true, false, `The log level, with 'debug' the most verbose and 'panic' the least verbose.
			One of: debug, info, warning, error, fatal, panic.
		`),
	},

	Options: []cmds.Option{
		cmds.StringOption("subsystem", "The subsystem logging identifier"),
		cmds.StringOption("expression", "Subsystem identifier by regular expression"),
	},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		level := strings.ToLower(req.Arguments[0])

		var s string
		if subsystem, ok := req.Options["subsystem"].(string); ok {
			if err := logging.SetLogLevel(subsystem, level); err != nil {
				return err
			}
			s = fmt.Sprintf("Changed log level of '%s' to '%s'", subsystem, level)
			loglogger.Info(s)
		} else if expression, ok := req.Options["expression"].(string); ok {
			if err := logging.SetLogLevelRegex(expression, level); err != nil {
				return err
			}
			s = fmt.Sprintf("Changed log level matching expression '%s' to '%s'", subsystem, level)
			loglogger.Info(s)
		} else {
			lvl, err := logging.LevelFromString(level)
			if err != nil {
				return err
			}
			logging.SetAllLoggers(lvl)
			s = fmt.Sprintf("Changed log level of all subsystems to: %s", level)
			loglogger.Info(s)
		}

		return cmds.EmitOnce(res, s)
	},
	Type: string(""),
}

var logLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List the logging subsystems.",
		ShortDescription: `
'venus log ls' is a utility command used to list the logging
subsystems of a running daemon.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		return cmds.EmitOnce(res, logging.GetSubsystems())
	},
	Type: []string{},
}
