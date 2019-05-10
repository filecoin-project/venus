package commands

import (
	"encoding/json"
	"io"
	"os"
	"runtime"

	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	sysi "github.com/whyrusleeping/go-sysinfo"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/flags"
	"github.com/filecoin-project/go-filecoin/repo"
)

var inspectCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
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
	Helptext: cmdkit.HelpText{
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
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, info *AllInspectorInfo) error {
			sw := NewSilentWriter(w)

			// Print Version
			sw.Printf("Version:\t%s\n", info.FilecoinVersion)

			// Print Runtime Info
			sw.Printf("\nRuntime\n")
			sw.Printf("OS:           \t%s\n", info.Runtime.OS)
			sw.Printf("Arch:         \t%s\n", info.Runtime.Arch)
			sw.Printf("Version:      \t%s\n", info.Runtime.Version)
			sw.Printf("Compiler:     \t%s\n", info.Runtime.Compiler)
			sw.Printf("NumProc:      \t%d\n", info.Runtime.NumProc)
			sw.Printf("GoMaxProcs:   \t%d\n", info.Runtime.GoMaxProcs)
			sw.Printf("NumGoRoutines:\t%d\n", info.Runtime.NumGoRoutines)
			sw.Printf("NumCGoCalls:  \t%d\n", info.Runtime.NumCGoCalls)

			// Print Disk Info
			sw.Printf("\nDisk\n")
			sw.Printf("Free:  \t%d\n", info.Disk.Free)
			sw.Printf("Total: \t%d\n", info.Disk.Total)
			sw.Printf("FSType:\t%s\n", info.Disk.FSType)

			// Print Memory Info
			sw.Printf("\nMemory\n")
			sw.Printf("Swap:   \t%d\n", info.Memory.Swap)
			sw.Printf("Virtual:\t%d\n", info.Memory.Virtual)

			// Print Environment Info
			sw.Printf("\nEnvironment\n")
			sw.Printf("FilAPI: \t%s\n", info.Environment.FilAPI)
			sw.Printf("FilPath:\t%s\n", info.Environment.FilPath)
			sw.Printf("GoPath: \t%s\n", info.Environment.GoPath)

			// Print Config Info
			sw.Printf("\nConfig\n")
			marshaled, err := json.MarshalIndent(info.Config, "", "\t")
			if err != nil {
				return err
			}
			sw.Printf("%s\n", marshaled)

			return sw.Error()
		}),
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
		out := GetInspectorAPI(env).Runtime()
		return cmds.EmitOnce(res, out)
	},
	Type: RuntimeInfo{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, info *RuntimeInfo) error {
			sw := NewSilentWriter(w)
			sw.Printf("OS:           \t%s\n", info.OS)
			sw.Printf("Arch:         \t%s\n", info.Arch)
			sw.Printf("Version:      \t%s\n", info.Version)
			sw.Printf("Compiler:     \t%s\n", info.Compiler)
			sw.Printf("NumProc:      \t%d\n", info.NumProc)
			sw.Printf("GoMaxProcs:   \t%d\n", info.GoMaxProcs)
			sw.Printf("NumGoRoutines:\t%d\n", info.NumGoRoutines)
			sw.Printf("NumCGoCalls:  \t%d\n", info.NumCGoCalls)
			return sw.Error()
		}),
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
		out, err := GetInspectorAPI(env).Disk()
		if err != nil {
			return err
		}
		return cmds.EmitOnce(res, out)
	},
	Type: DiskInfo{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, info *DiskInfo) error {
			sw := NewSilentWriter(w)
			sw.Printf("Free:  \t%d\n", info.Free)
			sw.Printf("Total: \t%d\n", info.Total)
			sw.Printf("FSType:\t%s\n", info.FSType)
			return sw.Error()
		}),
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
		out, err := GetInspectorAPI(env).Memory()
		if err != nil {
			return err
		}
		return cmds.EmitOnce(res, out)
	},
	Type: MemoryInfo{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, info *MemoryInfo) error {
			sw := NewSilentWriter(w)
			sw.Printf("Swap:   \t%d\n", info.Swap)
			sw.Printf("Virtual:\t%d\n", info.Virtual)
			return sw.Error()
		}),
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
		out := GetInspectorAPI(env).Config()
		return cmds.EmitOnce(res, out)
	},
	Type: config.Config{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, info *config.Config) error {
			marshaled, err := json.MarshalIndent(info, "", "\t")
			if err != nil {
				return err
			}
			marshaled = append(marshaled, byte('\n'))
			_, err = w.Write(marshaled)
			return err
		}),
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
		out := GetInspectorAPI(env).Environment()
		return cmds.EmitOnce(res, out)
	},
	Type: EnvironmentInfo{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, info *EnvironmentInfo) error {
			sw := NewSilentWriter(w)
			sw.Printf("FilAPI: \t%s\n", info.FilAPI)
			sw.Printf("FilPath:\t%s\n", info.FilPath)
			sw.Printf("GoPath: \t%s\n", info.GoPath)
			return sw.Error()
		}),
	},
}

// NewInspectorAPI returns a `Inspector` used to inspect the go-filecoin node.
func NewInspectorAPI(r repo.Repo) *Inspector {
	return &Inspector{
		repo: r,
	}
}

// Inspector contains information used to inspect the go-filecoin node.
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

// FilecoinVersion returns the version of go-filecoin.
func (g *Inspector) FilecoinVersion() string {
	return flags.Commit
}
