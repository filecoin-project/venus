package commands

import (
	"os"
	"runtime"

	cmds "github.com/ipfs/go-ipfs-cmds"
	sysi "github.com/whyrusleeping/go-sysinfo"

	"github.com/filecoin-project/venus/build/flags"
	"github.com/filecoin-project/venus/internal/pkg/config"
	"github.com/filecoin-project/venus/internal/pkg/repo"
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
		var allInfo AllInspectorInfo
		allInfo.Runtime = GetInspectorAPI(env).Runtime()

		dsk, err := GetInspectorAPI(env).Disk()
		if err != nil {
			return err
		}
		allInfo.Disk = dsk

		mem, err := GetInspectorAPI(env).Memory()
		if err != nil {
			return err
		}
		allInfo.Memory = mem
		allInfo.Config = GetInspectorAPI(env).Config()
		allInfo.Environment = GetInspectorAPI(env).Environment()
		allInfo.FilecoinVersion = GetInspectorAPI(env).FilecoinVersion()
		return cmds.EmitOnce(res, allInfo)
	},
	Type: AllInspectorInfo{},
}

var runtimeInspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print runtime diagnostic information.",
		ShortDescription: `
Prints out information about the golang runtime.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out := GetInspectorAPI(env).Runtime()
		return cmds.EmitOnce(res, out)
	},
	Type: RuntimeInfo{},
}

var diskInspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print filesystem usage information.",
		ShortDescription: `
Prints out information about the filesystem.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out, err := GetInspectorAPI(env).Disk()
		if err != nil {
			return err
		}
		return cmds.EmitOnce(res, out)
	},
	Type: DiskInfo{},
}

var memoryInspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print memory usage information.",
		ShortDescription: `
Prints out information about memory usage.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out, err := GetInspectorAPI(env).Memory()
		if err != nil {
			return err
		}
		return cmds.EmitOnce(res, out)
	},
	Type: MemoryInfo{},
}

var configInspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print in-memory config information.",
		ShortDescription: `
Prints out information about your filecoin nodes config.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		out := GetInspectorAPI(env).Config()
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
		out := GetInspectorAPI(env).Environment()
		return cmds.EmitOnce(res, out)
	},
	Type: EnvironmentInfo{},
}

// NewInspectorAPI returns a `Inspector` used to inspect the venus node.
func NewInspectorAPI(r repo.Repo) *Inspector {
	return &Inspector{
		repo: r,
	}
}

// Inspector contains information used to inspect the venus node.
type Inspector struct {
	repo repo.Repo
}

// AllInspectorInfo contains all information the inspector can gather.
type AllInspectorInfo struct {
	Config          *config.Config
	Runtime         *RuntimeInfo
	Environment     *EnvironmentInfo
	Disk            *DiskInfo
	Memory          *MemoryInfo
	FilecoinVersion string
}

// RuntimeInfo contains information about the golang runtime.
type RuntimeInfo struct {
	OS            string
	Arch          string
	Version       string
	Compiler      string
	NumProc       int
	GoMaxProcs    int
	NumGoRoutines int
	NumCGoCalls   int64
}

// EnvironmentInfo contains information about the environment filecoin is running in.
type EnvironmentInfo struct {
	FilAPI  string `json:"FIL_API"`
	FilPath string `json:"FIL_PATH"`
	GoPath  string `json:"GOPATH"`
}

// DiskInfo contains information about disk usage and type.
type DiskInfo struct {
	Free   uint64
	Total  uint64
	FSType string
}

// MemoryInfo contains information about memory usage.
type MemoryInfo struct {
	Swap    uint64
	Virtual uint64
}

// Runtime returns infrormation about the golang runtime.
func (g *Inspector) Runtime() *RuntimeInfo {
	return &RuntimeInfo{
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		Version:       runtime.Version(),
		Compiler:      runtime.Compiler,
		NumProc:       runtime.NumCPU(),
		GoMaxProcs:    runtime.GOMAXPROCS(0),
		NumGoRoutines: runtime.NumGoroutine(),
		NumCGoCalls:   runtime.NumCgoCall(),
	}
}

// Environment returns information about the environment filecoin is running in.
func (g *Inspector) Environment() *EnvironmentInfo {
	return &EnvironmentInfo{
		FilAPI:  os.Getenv("FIL_API"),
		FilPath: os.Getenv("FIL_PATH"),
		GoPath:  os.Getenv("GOPATH"),
	}
}

// Disk return information about filesystem the filecoin nodes repo is on.
func (g *Inspector) Disk() (*DiskInfo, error) {
	fsr, ok := g.repo.(*repo.FSRepo)
	if !ok {
		// we are using a in memory repo
		return &DiskInfo{
			Free:   0,
			Total:  0,
			FSType: "0",
		}, nil
	}

	p, err := fsr.Path()
	if err != nil {
		return nil, err
	}

	dinfo, err := sysi.DiskUsage(p)
	if err != nil {
		return nil, err
	}

	return &DiskInfo{
		Free:   dinfo.Free,
		Total:  dinfo.Total,
		FSType: dinfo.FsType,
	}, nil
}

// Memory return information about system meory usage.
func (g *Inspector) Memory() (*MemoryInfo, error) {
	meminfo, err := sysi.MemoryInfo()
	if err != nil {
		return nil, err
	}
	return &MemoryInfo{
		Swap:    meminfo.Swap,
		Virtual: meminfo.Used,
	}, nil
}

// Config return the current config values of the filecoin node.
func (g *Inspector) Config() *config.Config {
	return g.repo.Config()
}

// FilecoinVersion returns the version of venus.
func (g *Inspector) FilecoinVersion() string {
	return flags.GitCommit
}
