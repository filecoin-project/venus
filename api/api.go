// Package api holds the interface definitions for the Filecoin api.
package api

// API is the user interface to a Filecoin node.
type API interface {
	Address() Address
	Client() Client
	Daemon() Daemon
	Dag() Dag
	Ping() Ping
	RetrievalClient() RetrievalClient
	Swarm() Swarm
}
