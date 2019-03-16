package impl

import (
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/node"
)

type nodeAPI struct {
	node   *node.Node
	logger logging.EventLogger

	client *nodeClient
	daemon *nodeDaemon
	ping   *nodePing
	swarm  *nodeSwarm
}

// Assert that nodeAPI fullfills the api.API interface.
var _ api.API = (*nodeAPI)(nil)

// New constructs a new instance of the API.
func New(node *node.Node) api.API {
	api := &nodeAPI{
		node:   node,
		logger: logging.Logger("api"),
	}
	api.client = newNodeClient(api)
	api.daemon = newNodeDaemon(api)
	api.ping = newNodePing(api)
	api.swarm = newNodeSwarm(api)

	return api
}

func (api *nodeAPI) Client() api.Client {
	return api.client
}

func (api *nodeAPI) Daemon() api.Daemon {
	return api.daemon
}

func (api *nodeAPI) Ping() api.Ping {
	return api.ping
}

func (api *nodeAPI) Swarm() api.Swarm {
	return api.swarm
}
