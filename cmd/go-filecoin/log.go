package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log"
	writer "github.com/ipfs/go-log/writer"
)

var loglogger = logging.Logger("commands/log")

var logCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with the daemon event log output.",
		ShortDescription: `
'go-filecoin log' contains utility commands to affect the event logging
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
	Helptext: cmdkit.HelpText{
		Tagline: "Read the event log.",
		ShortDescription: `
Outputs event log messages (not other log messages) as they are generated.
`,
	},

	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		r, w := io.Pipe()
		go func() {
			defer w.Close() // nolint: errcheck
			<-req.Context.Done()
		}()

		writer.WriterGroup.AddWriter(w)

		return re.Emit(r)
	},
}

var logLevelCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Change the logging level.",
		ShortDescription: `
Change the verbosity of one or all subsystems log output. This does not affect
the event log.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("level", true, false, `The log level, with 'debug' the most verbose and 'panic' the least verbose.
			One of: debug, info, warning, error, fatal, panic.
		`),
	},

	Options: []cmdkit.Option{
		cmdkit.StringOption("subsystem", "The subsystem logging identifier"),
		cmdkit.StringOption("expression", "Subsystem identifier by regular expression"),
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
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out string) error {
			_, err := fmt.Fprint(w, out)
			return err
		}),
	},
}

var logLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List the logging subsystems.",
		ShortDescription: `
'go-filecoin log ls' is a utility command used to list the logging
subsystems of a running daemon.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		return cmds.EmitOnce(res, logging.GetSubsystems())
	},
	Type: []string{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, list []string) error {
			for _, s := range list {
				fmt.Fprintln(w, s) // nolint: errcheck
			}
			return nil
		}),
	},
}
