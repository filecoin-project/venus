// Package api holds the interface definitions for the Filecoin core api.
package api

// API is the unified interface to Filecoin on a Go level.
type API interface {
	Actor() Actor
	Address() Address
	Block() Block
	Bootstrap() Bootstrap
	Chain() Chain
	Config() Config
	Client() Client
	Daemon() Daemon
	Dag() Dag
	Id() Id
	Log() Log
	Message() Message
	Miner() Miner
	Mining() Mining
	Mpool() Mpool
	Orderbook() Orderbook
	Paych() Paych
	Ping() Ping
	Show() Show
	Swarm() Swarm
	Version() Version
}
