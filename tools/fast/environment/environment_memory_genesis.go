package environment

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
	"time"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	"github.com/filecoin-project/go-filecoin/tools/gengen/util"

	iptb "github.com/ipfs/iptb/testbed"
)

// MemoryGenesis is a FAST lib environment that is meant to be used
// when working locally, on the same network / machine. It's great for writing
// functional tests!
type MemoryGenesis struct {
	genesisCar        []byte
	genesisMinerOwner commands.WalletSerializeResult
	genesisMinerAddr  address.Address

	location string

	genesisServer     *http.Server
	genesisServerAddr string

	log logging.EventLogger

	processesMu sync.Mutex
	processes   []*fast.Filecoin

	processCountMu sync.Mutex
	processCount   int

	proofsMode types.ProofsMode
}

// NewMemoryGenesis builds an environment with a local genesis that can be used
// to initialize nodes and create a genesis node. The genesis file is provided by an http
// server.
func NewMemoryGenesis(funds *big.Int, location string, proofsMode types.ProofsMode) (Environment, error) {
	env := &MemoryGenesis{
		location:   location,
		log:        logging.Logger("environment"),
		proofsMode: proofsMode,
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

// GetFunds retrieves a fixed amount of tokens from the environment to the
// Filecoin processes default wallet address.
// GetFunds will cause the genesis node to send 1000 filecoin to process `p`.
func (e *MemoryGenesis) GetFunds(ctx context.Context, p *fast.Filecoin) error {
	e.log.Infof("GetFunds for process: %s", p.String())
	return series.SendFilecoinDefaults(ctx, e.Processes()[0], p, 1000)
}

// GenesisCar provides a url where the genesis file can be fetched from
func (e *MemoryGenesis) GenesisCar() string {
	uri := url.URL{
		Host:   e.genesisServerAddr,
		Path:   "genesis.car",
		Scheme: "http",
	}

	return uri.String()
}

// GenesisMiner provides required information to create a genesis node and
// load the wallet.
func (e *MemoryGenesis) GenesisMiner() (*GenesisMiner, error) {
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
func (e *MemoryGenesis) Log() logging.EventLogger {
	return e.log
}

// NewProcess builds a iptb process of the given type and options passed. The
// process is tracked by the environment and returned.
func (e *MemoryGenesis) NewProcess(ctx context.Context, processType string, options map[string]string, eo fast.FilecoinOpts) (*fast.Filecoin, error) {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()

	e.processCountMu.Lock()
	defer e.processCountMu.Unlock()

	ns := iptb.NodeSpec{
		Type:  processType,
		Dir:   fmt.Sprintf("%s/%d", e.location, e.processCount),
		Attrs: options,
	}
	e.processCount = e.processCount + 1

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
func (e *MemoryGenesis) Processes() []*fast.Filecoin {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()
	return e.processes[:]
}

// Teardown stops all of the nodes and cleans up the environment.
func (e *MemoryGenesis) Teardown(ctx context.Context) error {
	e.processesMu.Lock()
	defer e.processesMu.Unlock()

	e.log.Info("Teardown environment")
	for _, p := range e.processes {
		if err := p.StopDaemon(ctx); err != nil {
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
func (e *MemoryGenesis) TeardownProcess(ctx context.Context, p *fast.Filecoin) error {
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

// startGenesisServer builds and starts a server which will serve the genesis
// file, the url for the genesis.car is returned by GenesisCar()
func (e *MemoryGenesis) startGenesisServer() error {
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
func (e *MemoryGenesis) buildGenesis(funds *big.Int) error {
	cfg := &gengen.GenesisCfg{
		Keys: 1,
		PreAlloc: []string{
			funds.String(),
		},
		Miners: []*gengen.CreateStorageMinerConfig{
			{
				Owner:               0,
				NumCommittedSectors: 128,
			},
		},
		Network:    "go-filecoin-test",
		ProofsMode: e.proofsMode,
	}

	// ensure miners' sector size is set appropriately for the configured
	// proofs mode
	gengen.ApplyProofsModeDefaults(cfg, e.proofsMode == types.LiveProofsMode, true)

	var genbuffer bytes.Buffer

	info, err := gengen.GenGenesisCar(cfg, &genbuffer, 0, time.Unix(123456789, 0))
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
