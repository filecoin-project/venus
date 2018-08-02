// Package iface holds the interface definitions for the Filecoin core api.
package iface

import "github.com/filecoin-project/go-filecoin/core/node"

// CoreAPI is the unified interface to Filecoin on a Go level.
// TODO: decide on the name, this is what IPFS calls it
type CoreAPI interface {
	Actor() ActorAPI
	Address() AddressAPI
	Block() BlockAPI
	Bootstrap() BootstrapAPI
	Chain() ChainAPI
	Config() ConfigAPI
	Client() ClientAPI
	Daemon() DaemonAPI
	Dag() DagAPI
	Id() IdAPI
	Init() InitAPI
	Log() LogAPI
	Message() MessageAPI
	Miner() MinerAPI
	Mining() MiningAPI
	Mpool() MpoolAPI
	Orderbook() OrderbookAPI
	Paych() PaychAPI
	Ping() PingAPI
	Show() ShowAPI
	Swarm() SwarmAPI
	Version() VersionAPI
	Wallet() WalletAPI

	// DEPRECATED
	// TODO: remove once all commands are migrated to the new api
	Node() *node.Node
}
