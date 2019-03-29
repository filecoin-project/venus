package fast

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"

	cid "github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/address"

	iptb "github.com/ipfs/iptb/testbed"
)

// EnvironmentDevnet is a FAST lib environment that is meant to be used
// when working with kittyhawk devnets.
type EnvironmentDevnet struct {
	network  string
	location string

	log logging.EventLogger

	processesMu sync.Mutex
	processes   []*Filecoin
}

// NewEnvironmentDevnet builds an environment that uses deployed infrastructure to
// the kittyhawk devnets.
func NewEnvironmentDevnet(network, location string) (Environment, error) {
	env := &EnvironmentDevnet{
		network:  network,
		location: location,
		log:      logging.Logger("environment"),
	}

	if err := os.MkdirAll(env.location, 0775); err != nil {
		return nil, err
	}

	return env, nil
}

// GenesisCar provides a url where the genesis file can be fetched from
func (e *EnvironmentDevnet) GenesisCar() string {
	uri := url.URL{
		Host:   fmt.Sprintf("genesis.%s.kittyhawk.wtf", e.network),
		Path:   "genesis.car",
		Scheme: "https",
	}

	return uri.String()
}

// GenesisMiner returns a ErrNoGenesisMiner for this environment
func (e *EnvironmentDevnet) GenesisMiner() (*GenesisMiner, error) {
	return nil, ErrNoGenesisMiner
}

// Log returns the logger for the environment.
func (e *EnvironmentDevnet) Log() logging.EventLogger {
	return e.log
}

// NewProcess builds a iptb process of the given type and options passed. The
// process is tracked by the environment and returned.
func (e *EnvironmentDevnet) NewProcess(ctx context.Context, processType string, options map[string]string, eo EnvironmentOpts) (*Filecoin, error) {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()

	ns := iptb.NodeSpec{
		Type:  processType,
		Dir:   fmt.Sprintf("%s/%d", e.location, len(e.processes)),
		Attrs: options,
	}

	e.log.Infof("New Process type: %s, dir: %s", processType, ns.Dir)

	if err := os.MkdirAll(ns.Dir, 0775); err != nil {
		return nil, err
	}

	c, err := ns.Load()
	if err != nil {
		return nil, err
	}

	// We require a slightly more extended core interface
	fc, ok := c.(IPTBCoreExt)
	if !ok {
		return nil, fmt.Errorf("%s does not implement the extended IPTB.Core interface IPTBCoreExt", processType)
	}

	p := NewFilecoinProcess(ctx, fc, eo)
	e.processes = append(e.processes, p)
	return p, nil
}

// Processes returns all processes the environment knows about.
func (e *EnvironmentDevnet) Processes() []*Filecoin {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()
	return e.processes[:]
}

// Teardown stops all of the nodes and cleans up the environment.
func (e *EnvironmentDevnet) Teardown(ctx context.Context) error {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()

	e.log.Info("Teardown environment")
	for _, p := range e.processes {
		if err := p.core.Stop(ctx); err != nil {
			return err
		}
	}

	return os.RemoveAll(e.location)
}

// TeardownProcess stops the running process and removes it from the
// environment.
func (e *EnvironmentDevnet) TeardownProcess(ctx context.Context, p *Filecoin) error {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()

	e.log.Infof("Teardown process: %s", p.core.String())
	if err := p.core.Stop(ctx); err != nil {
		return err
	}

	for i, n := range e.processes {
		if n == p {
			e.processes = append(e.processes[:i], e.processes[i+1:]...)
			break
		}
	}

	// remove the provess from the process list
	return os.RemoveAll(p.core.Dir())
}

// GetFunds retrieves a fixed amount of tokens from an environment
func GetFunds(ctx context.Context, env Environment, p *Filecoin) error {
	switch devenv := env.(type) {
	case *EnvironmentDevnet:
		var toAddr address.Address
		if err := p.ConfigGet(ctx, "wallet.defaultAddress", &toAddr); err != nil {
			return err
		}

		data := url.Values{}
		data.Set("target", toAddr.String())

		uri := url.URL{
			Host:   fmt.Sprintf("faucet.%s.kittyhawk.wtf", devenv.network),
			Path:   "tap",
			Scheme: "https",
		}

		resp, err := http.PostForm(uri.String(), data)
		if err != nil {
			return err
		}

		msgcid := resp.Header.Get("Message-Cid")
		mcid, err := cid.Decode(msgcid)
		if err != nil {
			return err
		}

		if _, err := p.MessageWait(ctx, mcid); err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("environment does not support GetFunds")
}
