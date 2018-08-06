// Package api holds the interface definitions for the Filecoin core api.
package api

// API is the unified interface to Filecoin on a Go level.
type API interface {
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
}
