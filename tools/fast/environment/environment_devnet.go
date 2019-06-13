package environment

// The devnet FAST environment provides an environment for using FAST with the deployed kittyhawk
// devnet infrasturture run by the Filecoin development team. It can be used to setup and manage nodes
// connected to either the nightly, test, or user devnets for running automation with the FAST library.

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sync"

	cid "github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	iptb "github.com/ipfs/iptb/testbed"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// Devnet is a FAST lib environment that is meant to be used
// when working with kittyhawk devnets run by the Filecoin development team.
type Devnet struct {
	network  string
	location string

	log logging.EventLogger

	processesMu sync.Mutex
	processes   []*fast.Filecoin
}

// NewDevnet builds an environment that uses deployed infrastructure to
// the kittyhawk devnets.
func NewDevnet(network, location string) (Environment, error) {
	env := &Devnet{
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
func (e *Devnet) GenesisCar() string {
	uri := url.URL{
		Host:   fmt.Sprintf("genesis.%s.kittyhawk.wtf", e.network),
		Path:   "genesis.car",
		Scheme: "https",
	}

	return uri.String()
}

// GenesisMiner returns a ErrNoGenesisMiner for this environment
func (e *Devnet) GenesisMiner() (*GenesisMiner, error) {
	return nil, ErrNoGenesisMiner
}

// Log returns the logger for the environment.
func (e *Devnet) Log() logging.EventLogger {
	return e.log
}

// NewProcess builds a iptb process of the given type and options passed. The
// process is tracked by the environment and returned.
func (e *Devnet) NewProcess(ctx context.Context, processType string, options map[string]string, eo fast.FilecoinOpts) (*fast.Filecoin, error) {
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
	fc, ok := c.(fast.IPTBCoreExt)
	if !ok {
		return nil, fmt.Errorf("%s does not implement the extended IPTB.Core interface IPTBCoreExt", processType)
	}

	p := fast.NewFilecoinProcess(ctx, fc, eo)
	e.processes = append(e.processes, p)
	return p, nil
}

// Processes returns all processes the environment knows about.
func (e *Devnet) Processes() []*fast.Filecoin {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()
	return e.processes[:]
}

// Teardown stops all of the nodes and cleans up the environment.
func (e *Devnet) Teardown(ctx context.Context) error {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()

	e.log.Info("Teardown environment")
	for _, p := range e.processes {
		if err := p.StopDaemon(ctx); err != nil {
			return err
		}
	}

	return os.RemoveAll(e.location)
}

// TeardownProcess stops the running process and removes it from the
// environment.
func (e *Devnet) TeardownProcess(ctx context.Context, p *fast.Filecoin) error {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()

	e.log.Infof("Teardown process: %s", p.String())
	if err := p.StopDaemon(ctx); err != nil {
		return err
	}

	for i, n := range e.processes {
		if n == p {
			e.processes = append(e.processes[:i], e.processes[i+1:]...)
			break
		}
	}

	// remove the provess from the process list
	return os.RemoveAll(p.Dir())
}

// GetFunds retrieves a fixed amount of tokens from the environment to the
// Filecoin processes default wallet address.
// GetFunds will send a request to the Faucet, the amount of tokens returned and
// number of requests permitted is determined by the Faucet configuration.
func (e *Devnet) GetFunds(ctx context.Context, p *fast.Filecoin) error {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()

	e.log.Infof("GetFunds for process: %s", p.String())
	var toAddr address.Address
	if err := p.ConfigGet(ctx, "wallet.defaultAddress", &toAddr); err != nil {
		return err
	}

	data := url.Values{}
	data.Set("target", toAddr.String())

	uri := url.URL{
		Host:   fmt.Sprintf("faucet.%s.kittyhawk.wtf", e.network),
		Path:   "tap",
		Scheme: "https",
	}

	resp, err := http.PostForm(uri.String(), data)
	if err != nil {
		return err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case 200:
		msgcid := resp.Header.Get("Message-Cid")
		mcid, err := cid.Decode(msgcid)
		if err != nil {
			return err
		}

		if _, err := p.MessageWait(ctx, mcid); err != nil {
			return err
		}
		return nil
	case 400:
		return fmt.Errorf("Bad Request: %s", string(b))
	case 429:
		return fmt.Errorf("Rate Limit: %s", string(b))
	default:
		return fmt.Errorf("Unhandled Status: %s", resp.Status)
	}
}
