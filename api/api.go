// Package api holds the interface definitions for the Filecoin api.
package api

import (
	"github.com/filecoin-project/go-filecoin/node/sectorforeman"
)

// API is the user interface to a Filecoin node.
type API interface {
	Actor() Actor
	Address() Address
	Client() Client
	Daemon() Daemon
	Dag() Dag
<<<<<<< HEAD
	Miner() Miner
=======
	Mining() Mining
>>>>>>> Extracted Miner Create method from the node api to the miner create command
	Ping() Ping
	RetrievalClient() RetrievalClient
	Swarm() Swarm
}
