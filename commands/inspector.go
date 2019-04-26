package commands

import (
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var inspectCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show info about the filecoin node",
	},
	Subcommands: map[string]*cmds.Command{
		"runtime":     runtimeInspectCmd,
		"disk":        diskInspectCmd,
		"memory":      memoryInspectCmd,
		"config":      configInspectCmd,
		"environment": envInspectCmd,
	},
}

var runtimeInspectCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Print runtime diagnostic information.",
		ShortDescription: `
Prints out information about the golang runtime.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out := GetPorcelainAPI(env).Inspector().Runtime()
		return cmds.EmitOnce(res, out)
	},
}

var diskInspectCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Print filesystem usage information.",
		ShortDescription: `
Prints out information about the filesystem.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out, err := GetPorcelainAPI(env).Inspector().Disk()
		if err != nil {
			return err
		}
		return cmds.EmitOnce(res, out)
	},
}

var memoryInspectCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Print memory usage information.",
		ShortDescription: `
Prints out information about memory usage.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out, err := GetPorcelainAPI(env).Inspector().Memory()
		if err != nil {
			return err
		}
		return cmds.EmitOnce(res, out)
	},
}

var configInspectCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Print in-memory config information.",
		ShortDescription: `
Prints out information about your filecoin nodes config.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out := GetPorcelainAPI(env).Inspector().Config()
		return cmds.EmitOnce(res, out)
	},
}

var envInspectCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Print filecoin environment information.",
		ShortDescription: `
Prints out information about your filecoin nodes environment.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out := GetPorcelainAPI(env).Inspector().Environment()
		return cmds.EmitOnce(res, out)
	},
}
