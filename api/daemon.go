package api

import (
	"context"

	"github.com/filecoin-project/go-filecoin/address"
)

// Daemon is the interface that defines methods to change the state of the daemon.
type Daemon interface {
	// Stop, shuts down the daemon and cleans up any resources.
	Stop(ctx context.Context) error
	// Init, initializes everything needed to run a daemon, including the disk storage.
	Init(ctx context.Context, opts ...DaemonInitOpt) error
}

// DaemonInitConfig is a helper struct to configure the init process of a daemon.
type DaemonInitConfig struct {
	// GenesisFile, path to a file containing archive of genesis block DAG data
	GenesisFile string
	// RepoDir, path to the repo of the node on disk.
	RepoDir string
	// PeerKeyFile is the path to a file containing a libp2p peer id key
	PeerKeyFile string
	// WithMiner, if set, sets the config value for the local miner to this address.
	WithMiner address.Address
	// DevnetTest, if set, sets the config to enable bootstrapping to the test devnet.
	DevnetTest bool
	// DevnetNightly, if set, sets the config to enable bootstrapping to the nightly devnet.
	DevnetNightly bool
	// DevnetUser, if set, sets the config to enable bootstrapping to the user devnet.
	DevnetUser bool
	// AutoSealIntervalSeconds, when set, configures the daemon to check for and seal any staged sectors on an interval
	AutoSealIntervalSeconds uint
	DefaultAddress          address.Address
}

// DaemonInitOpt is the signature a daemon init option has to fulfill.
type DaemonInitOpt func(*DaemonInitConfig)

// GenesisFile defines a custom genesis file to use on daemon init.
func GenesisFile(p string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.GenesisFile = p
	}
}

// RepoDir defines the location on disk of the repo.
func RepoDir(p string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.RepoDir = p
	}
}

// PeerKeyFile defines the file to load a libp2p peer key from
func PeerKeyFile(p string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.PeerKeyFile = p
	}
}

// WithMiner sets the WithMiner option.
func WithMiner(miner address.Address) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.WithMiner = miner
	}
}

// DevnetTest sets the DevnetTest option.
func DevnetTest(doit bool) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.DevnetTest = doit
	}
}

// DevnetNightly sets the DevnetNightly option.
func DevnetNightly(doit bool) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.DevnetNightly = doit
	}
}

// DevnetUser sets the DevnetUser option.
func DevnetUser(doit bool) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.DevnetUser = doit
	}
}

// AutoSealIntervalSeconds configures the daemon to check for and seal any staged sectors on an interval.
func AutoSealIntervalSeconds(autoSealIntervalSeconds uint) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.AutoSealIntervalSeconds = autoSealIntervalSeconds
	}
}

// DefaultAddress sets the daemons's default address to the provided address.
// When not used, node/init.go Init generates a new address in the wallet and sets it to
// the default address. Use this if you want a specific DefaultAddress.
func DefaultAddress(address address.Address) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.DefaultAddress = address
	}
}
