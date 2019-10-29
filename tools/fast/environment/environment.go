package environment

import (
	"context"
	"errors"
	"io"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// ErrNoGenesisMiner is returned by GenesisMiner if the environment does not
// support providing a genesis miner.
var ErrNoGenesisMiner = errors.New("GenesisMiner not supported")

// GenesisMiner contains the required information to setup a node as a genesis
// node.
type GenesisMiner struct {
	// Address is the address of the miner on chain
	Address address.Address

	// Owner is the private key of the wallet which is assoiated with the miner
	Owner io.Reader
}

// Environment defines the interface common among all environments that the
// FAST lib can work across. It helps smooth out the differences by providing
// a common ground to work from
type Environment interface {
	// GenesisCar returns a location to the genesis.car file. This can be
	// either an absolute path to a file on disk, or more commonly an http(s)
	// url.
	GenesisCar() string

	// GenesisMiner returns a structure which contains all the required
	// information to load the existing miner that is defined in the
	// genesis block. An ErrNoGenesisMiner may be returned if the environment
	// does not support providing genesis miner information.
	GenesisMiner() (*GenesisMiner, error)

	// Log returns a logger for the environment
	Log() logging.EventLogger

	// NewProcess makes a new process for the environment. This doesn't
	// always mean a new filecoin node though, NewProcess for some
	// environments may create a Filecoin process that interacts with
	// an already running filecoin node, and supplied the API multiaddr
	// as options.
	NewProcess(ctx context.Context, processType string, options map[string]string, eo fast.FilecoinOpts) (*fast.Filecoin, error)

	// Processes returns a slice of all processes the environment knows
	// about.
	Processes() []*fast.Filecoin

	// Teardown runs anything that the environment may need to do to
	// be nice to the the execution area of this code.
	Teardown(context.Context) error

	// TeardownProcess runs anything that the environment may need to do
	// to remove a process from the environment in a clean way.
	TeardownProcess(context.Context, *fast.Filecoin) error

	// GetFunds retrieves a fixed amount of tokens from the environment to the
	// Filecoin processes default wallet address.
	GetFunds(context.Context, *fast.Filecoin) error
}
