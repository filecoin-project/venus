// Package api holds the interface definitions for the Filecoin api.
package api

// API is the user interface to a Filecoin node.
type API interface {
	Client() Client
	Daemon() Daemon
	Ping() Ping
	RetrievalClient() RetrievalClient
	Swarm() Swarm
}
