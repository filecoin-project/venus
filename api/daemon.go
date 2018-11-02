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
type DaemonInitOpt func(*DaemonInitConfig) error

// UseCustomGenesis enables or disables the custom genesis functionality on daemon init.
func UseCustomGenesis(use bool) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.UseCustomGenesis = use
		return nil
	}
}

// WalletFile sets the path to a wallet file on daemon init.
func WalletFile(p string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.WalletFile = p
		return nil
	}
}

// WalletAddr defines a the address to store, used in combination with WalletFile.
func WalletAddr(addr string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.WalletAddr = addr
		return nil
	}
}

// GenesisFile defines a custom genesis file to use on daemon init.
func GenesisFile(p string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.GenesisFile = p
		return nil
	}
}

// RepoDir defines the location on disk of the repo.
func RepoDir(p string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.RepoDir = p
		return nil
	}
}

// PeerKeyFile defines the file to load a libp2p peer key from
func PeerKeyFile(p string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.PeerKeyFile = p
		return nil
	}
}

// WithMiner sets the WithMiner option.
func WithMiner(miner address.Address) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.WithMiner = miner
		return nil
	}
}

// LabWeekCluster sets the LabWeekCluster option.
func LabWeekCluster(doit bool) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.LabWeekCluster = doit
		return nil
	}
}

// AutoSealIntervalSeconds configures the daemon to check for and seal any staged sectors on an interval.
func AutoSealIntervalSeconds(autoSealIntervalSeconds uint) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.AutoSealIntervalSeconds = autoSealIntervalSeconds
		return nil
	}
}
