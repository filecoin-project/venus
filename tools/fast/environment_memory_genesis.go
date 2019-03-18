package fast

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"

	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/commands"
	"github.com/filecoin-project/go-filecoin/gengen/util"

	iptb "github.com/ipfs/iptb/testbed"
)

// EnvironmentMemoryGenesis is a FAST lib environment that is meant to be used
// when working locally, on the same network / machine. It's great for writing
// functional tests!
type EnvironmentMemoryGenesis struct {
	genesisCar        []byte
	genesisMinerOwner commands.WalletSerializeResult
	genesisMinerAddr  address.Address

	location string

	genesisServer     *http.Server
	genesisServerAddr string

	log logging.EventLogger

	processesMu sync.Mutex
	processes   []*Filecoin
}

// NewEnvironmentMemoryGenesis builds an environment with a local genesis that can be used
// to initialize nodes and create a genesis node. The genesis file is provided by an http
// server.
func NewEnvironmentMemoryGenesis(funds *big.Int, location string) (Environment, error) {
	env := &EnvironmentMemoryGenesis{
		location: location,
		log:      logging.Logger("environment"),
	}

	if err := env.buildGenesis(funds); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(env.location, 0775); err != nil {
		return nil, err
	}

	if err := env.startGenesisServer(); err != nil {
		return nil, err
	}

	return env, nil
}

// GenesisCar provides a url where the genesis file can be fetched from
func (e *EnvironmentMemoryGenesis) GenesisCar() string {
	uri := url.URL{
		Host:   e.genesisServerAddr,
		Path:   "genesis.car",
		Scheme: "http",
	}

	return uri.String()
}

// GenesisMiner provides required information to create a genesis node and
// load the wallet.
func (e *EnvironmentMemoryGenesis) GenesisMiner() (*GenesisMiner, error) {
	owner, err := json.Marshal(e.genesisMinerOwner)
	if err != nil {
		return nil, err
	}

	return &GenesisMiner{
		Address: e.genesisMinerAddr,
		Owner:   bytes.NewBuffer(owner),
	}, nil
}

// Log returns the logger for the environment.
func (e *EnvironmentMemoryGenesis) Log() logging.EventLogger {
	return e.log
}

// NewProcess builds a iptb process of the given type and options passed. The
// process is tracked by the environment and returned.
func (e *EnvironmentMemoryGenesis) NewProcess(ctx context.Context, processType string, options map[string]string, eo EnvironmentOpts) (*Filecoin, error) {
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
func (e *EnvironmentMemoryGenesis) Processes() []*Filecoin {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()
	return e.processes[:]
}

// Teardown stops all of the nodes and cleans up the environment.
func (e *EnvironmentMemoryGenesis) Teardown(ctx context.Context) error {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()

	e.log.Info("Teardown environment")
	for _, p := range e.processes {
		if err := p.core.Stop(ctx); err != nil {
			return err
		}
	}

	if err := e.genesisServer.Shutdown(ctx); err != nil {
		return err
	}

	return os.RemoveAll(e.location)
}

// TeardownProcess stops the running process and removes it from the
// environment.
func (e *EnvironmentMemoryGenesis) TeardownProcess(ctx context.Context, p *Filecoin) error {
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

// startGenesisServer builds and starts a server which will serve the genesis
// file, the url for the genesis.car is returned by GenesisCar()
func (e *EnvironmentMemoryGenesis) startGenesisServer() error {
	handler := http.NewServeMux()
	handler.HandleFunc("/genesis.car", func(w http.ResponseWriter, req *http.Request) {
		car := bytes.NewBuffer(e.genesisCar)
		if n, err := io.Copy(w, car); err != nil {
			e.log.Errorf(`Failed to serve "/genesis.car" after writing %d bytes with error %s`, n, err)
		}
	})

	e.genesisServer = &http.Server{Handler: handler}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	e.genesisServerAddr = ln.Addr().String()

	go func() {
		if err := e.genesisServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			e.log.Errorf("Genesis file server: %s", err)
		}
	}()

	return nil
}

// buildGenesis builds a genesis with the specified funds.
func (e *EnvironmentMemoryGenesis) buildGenesis(funds *big.Int) error {
	cfg := &gengen.GenesisCfg{
		Keys: 1,
		PreAlloc: []string{
			funds.String(),
		},
		Miners: []gengen.Miner{
			{
				Owner: 0,
				Power: 1,
			},
		},
	}

	var genbuffer bytes.Buffer

	info, err := gengen.GenGenesisCar(cfg, &genbuffer, 0)
	if err != nil {
		return err
	}

	if len(info.Keys) == 0 {
		return fmt.Errorf("no key was generated")
	}

	if len(info.Miners) == 0 {
		return fmt.Errorf("no miner was generated")
	}

	e.genesisCar = genbuffer.Bytes()
	e.genesisMinerOwner = commands.WalletSerializeResult{KeyInfo: info.Keys}
	e.genesisMinerAddr = info.Miners[0].Address

	return nil
}
