package api

import (
	"context"
)

type Daemon interface {
	// Start, starts a new daemon process.
	Start(ctx context.Context) error
	// Stop, shuts down the daemon and cleans up any resources.
	Stop(ctx context.Context) error
	// Init, initializes everything needed to run a daemon, including the disk storage.
	Init(ctx context.Context, opts ...DaemonInitOpt) error
}

// DaemonInitConfig, his a helper struct to configure the init process of a daemon.
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
}

type DaemonInitOpt func(*DaemonInitConfig) error

// UseCustomGenesis, enables or disables the custom genesis functionality on daemon init.
func UseCustomGenesis(use bool) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.UseCustomGenesis = use
		return nil
	}
}

func WalletFile(p string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.WalletFile = p
		return nil
	}
}

func WalletAddr(addr string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.WalletAddr = addr
		return nil
	}
}

func GenesisFile(p string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.GenesisFile = p
		return nil
	}

}
func RepoDir(p string) DaemonInitOpt {
	return func(dc *DaemonInitConfig) error {
		dc.RepoDir = p
		return nil
	}
}
