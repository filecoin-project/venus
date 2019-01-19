package fat

import (
	"context"
	"fmt"
	"os"
	"sync"

	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"

	iptb "github.com/ipfs/iptb/testbed"
)

// Environment is a structure which contains a set of filecoin processes
// and globally shared resources.
type Environment struct {
	Location string

	GenesisFile *GenesisInfo
	GenesisNode *Filecoin

	Log logging.EventLogger

	procMu    sync.Mutex
	processes []*Filecoin
}

// NewEnvironment creates a new Environment and genesis file for the environment.
func NewEnvironment(location string) (*Environment, error) {
	gf, err := GenerateGenesis(10000000, location)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(location, 0775); err != nil {
		return nil, err
	}

	return &Environment{
		Location:    location,
		GenesisFile: gf,
		Log:         logging.Logger("environment"),
		processes:   make([]*Filecoin, 0),
	}, nil
}

func (e *Environment) addProcess(p *Filecoin) {
	e.procMu.Lock()
	defer e.procMu.Unlock()

	e.processes = append(e.processes, p)
}

func (e *Environment) removeProcess(p *Filecoin) {
	e.procMu.Lock()
	defer e.procMu.Unlock()

	for i, n := range e.processes {
		if n == p {
			e.processes = append(e.processes[:i], e.processes[i+1:]...)
			return
		}
	}

}

// Processes returns the managed by the environment.
func (e *Environment) Processes() []*Filecoin {
	e.procMu.Lock()
	defer e.procMu.Unlock()

	return e.processes
}

// Teardown stops all processes managed by the environment and cleans up the
// location the environment was running in.
func (e *Environment) Teardown(ctx context.Context) error {
	e.procMu.Lock()
	defer e.procMu.Unlock()

	e.Log.Info("Teardown environment")
	for _, p := range e.processes {
		if err := p.core.Stop(ctx); err != nil {
			return err
		}
	}

	return os.RemoveAll(e.Location)
}

// NewProcess creates a new Filecoin process of type `processType`, with attributes `attrs`.
func (e *Environment) NewProcess(ctx context.Context, processType string, attrs map[string]string) (*Filecoin, error) {
	ns := iptb.NodeSpec{
		Type:  processType,
		Dir:   fmt.Sprintf("%s/%d", e.Location, len(e.processes)),
		Attrs: attrs,
	}
	e.Log.Infof("New Process type: %s, dir: %s", processType, ns.Dir)

	if err := os.MkdirAll(ns.Dir, 0775); err != nil {
		return nil, err
	}

	c, err := ns.Load()
	if err != nil {
		return nil, err
	}

	p := NewFilecoinProcess(ctx, c)
	e.addProcess(p)
	return p, nil
}

// TeardownProcess stops process `p`, and cleans up the location the process was running in.
func (e *Environment) TeardownProcess(ctx context.Context, p *Filecoin) error {
	e.Log.Infof("Teardown process: %s", p.core.String())
	if err := p.core.Stop(ctx); err != nil {
		return err
	}

	// remove the provess from the process list
	e.removeProcess(p)
	return os.RemoveAll(p.core.Dir())
}

// ConnectProcess connects process `p` to all other processes in the environment.
func (e *Environment) ConnectProcess(ctx context.Context, p *Filecoin) error {
	e.Log.Infof("Connect process: %s", p.core.String())
	if !p.IsAlve {
		return fmt.Errorf("process is not running, cannot connect to environment")
	}
	for _, p := range e.processes {
		if err := p.core.Connect(ctx, p.core); err != nil {
			return err
		}
	}
	return nil
}
