package api

import (
	"context"

	"github.com/filecoin-project/go-filecoin/address"
)

// Daemon is the interface that defines methods to change the state of the daemon.
type Daemon interface {
	// Start, starts a new daemon process.
	Start(ctx context.Context) error
	// Stop, shuts down the daemon and cleans up any resources.
	Stop(ctx context.Context) error
	// Init, initializes everything needed to run a daemon, including the disk storage.
	Init(ctx context.Context, opts ...DaemonInitOpt) error
}

// DaemonInitConfig is a helper struct to configure the init process of a daemon.
type DaemonInitConfig struct {
	// WalletFile, path to a file that contains addresses and private keys.
	WalletFile string
	// WalletAddr, the address to store when a WalletFile is given.
	WalletAddr string
	// GenesisFile, path to a file containing archive of genesis block DAG data
	GenesisFile string
	// UseCustomGenesis, when set, the init process creates a custom genesis block with pre-mined funds.
	UseCustomGenesis bool
	// RepoDir, path to the repo of the node on disk.
	RepoDir string
	// PeerKeyFile is the path to a file containing a libp2p peer id key
	PeerKeyFile string
	// WithMiner, if set, sets the config value for the local miner to this address.
	WithMiner address.Address
	// LabWeekCluster, if set, sets the config to enable bootstrapping to the labweek cluster.
	LabWeekCluster bool
	// AutoSealIntervalSeconds, when set, configures the daemon to check for and seal any staged sectors on an interval
	AutoSealIntervalSeconds uint
}

// DaemonInitOpt is the signature a daemon init option has to fulfill.
type DaemonInitOpt func(*DaemonInitConfig)

// UseCustomGenesis enables or disables the custom genesis functionality on daemon init.
func UseCustomGenesis(use bool) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.UseCustomGenesis = use
	}
}

// WalletFile sets the path to a wallet file on daemon init.
func WalletFile(p string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.WalletFile = p
	}
}

// WalletAddr defines a the address to store, used in combination with WalletFile.
func WalletAddr(addr string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.WalletAddr = addr
	}
}

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

// LabWeekCluster sets the LabWeekCluster option.
func LabWeekCluster(doit bool) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.LabWeekCluster = doit
	}
}

// AutoSealIntervalSeconds configures the daemon to check for and seal any staged sectors on an interval.
func AutoSealIntervalSeconds(autoSealIntervalSeconds uint) DaemonInitOpt {
	return func(dc *DaemonInitConfig) {
		dc.AutoSealIntervalSeconds = autoSealIntervalSeconds
	}
}
