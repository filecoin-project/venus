package node

import (
	"fmt"
	"github.com/filecoin-project/venus/build/flags"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/repo"
	sysi "github.com/whyrusleeping/go-sysinfo"
	"os"
	"runtime"
)

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

// configModule return the current config values of the filecoin node.
func (g *Inspector) Config() *config.Config {
	return g.repo.Config()
}

// FilecoinVersion returns the version of venus.
func (g *Inspector) FilecoinVersion() string {
	return fmt.Sprintf("%s %s", flags.GitTag, flags.GitCommit)
}
