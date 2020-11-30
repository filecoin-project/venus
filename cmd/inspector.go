package cmd

import (
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/config"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var inspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show info about the filecoin node",
	},
	Subcommands: map[string]*cmds.Command{
		"all":         allInspectCmd,
		"runtime":     runtimeInspectCmd,
		"disk":        diskInspectCmd,
		"memory":      memoryInspectCmd,
		"config":      configInspectCmd,
		"environment": envInspectCmd,
	},
}
var allInspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print all diagnostic information.",
		ShortDescription: `
Prints out information about filecoin process and its environment.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		var allInfo node.AllInspectorInfo
		allInfo.Runtime = env.(*node.Env).InspectorAPI.Runtime()

		dsk, err := env.(*node.Env).InspectorAPI.Disk()
		if err != nil {
			return err
		}
		allInfo.Disk = dsk

		mem, err := env.(*node.Env).InspectorAPI.Memory()
		if err != nil {
			return err
		}
		allInfo.Memory = mem
		allInfo.Config = env.(*node.Env).InspectorAPI.Config()
		allInfo.Environment = env.(*node.Env).InspectorAPI.Environment()
		allInfo.FilecoinVersion = env.(*node.Env).InspectorAPI.FilecoinVersion()
		return cmds.EmitOnce(res, allInfo)
	},
	Type: node.AllInspectorInfo{},
}

var runtimeInspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print runtime diagnostic information.",
		ShortDescription: `
Prints out information about the golang runtime.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out := env.(*node.Env).InspectorAPI.Runtime()
		return cmds.EmitOnce(res, out)
	},
	Type: node.RuntimeInfo{},
}

var diskInspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print filesystem usage information.",
		ShortDescription: `
Prints out information about the filesystem.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out, err := env.(*node.Env).InspectorAPI.Disk()
		if err != nil {
			return err
		}
		return cmds.EmitOnce(res, out)
	},
	Type: node.DiskInfo{},
}

var memoryInspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print memory usage information.",
		ShortDescription: `
Prints out information about memory usage.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out, err := env.(*node.Env).InspectorAPI.Memory()
		if err != nil {
			return err
		}
		return cmds.EmitOnce(res, out)
	},
	Type: node.MemoryInfo{},
}

var configInspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print in-memory config information.",
		ShortDescription: `
Prints out information about your filecoin nodes config.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out := env.(*node.Env).InspectorAPI.Config()
		return cmds.EmitOnce(res, out)
	},
	Type: config.Config{},
}

var envInspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print filecoin environment information.",
		ShortDescription: `
Prints out information about your filecoin nodes environment.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out := env.(*node.Env).InspectorAPI.Environment()
		return cmds.EmitOnce(res, out)
	},
	Type: node.EnvironmentInfo{},
}
