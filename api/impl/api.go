package impl

import (
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/porcelain"
)

type nodeAPI struct {
	node   *node.Node
	logger logging.EventLogger

	address         *nodeAddress
	client          *nodeClient
	daemon          *nodeDaemon
	dag             *nodeDag
	miner           *nodeMiner
	ping            *nodePing
	retrievalClient *nodeRetrievalClient
	swarm           *nodeSwarm
}

// Assert that nodeAPI fullfills the api.API interface.
var _ api.API = (*nodeAPI)(nil)

// New constructs a new instance of the API.
func New(node *node.Node) api.API {
	api := &nodeAPI{
		node:   node,
		logger: logging.Logger("api"),
	}
	var porcelainAPI *porcelain.API
	if node != nil {
		porcelainAPI = node.PorcelainAPI
	}

	api.address = newNodeAddress(api)
	api.client = newNodeClient(api)
	api.daemon = newNodeDaemon(api)
	api.dag = newNodeDag(api)
	api.miner = newNodeMiner(api, porcelainAPI)
	api.ping = newNodePing(api)
	api.retrievalClient = newNodeRetrievalClient(api, porcelainAPI)
	api.swarm = newNodeSwarm(api)

	return api
}

func (api *nodeAPI) Address() api.Address {
	return api.address
}

func (api *nodeAPI) Client() api.Client {
	return api.client
}

func (api *nodeAPI) Daemon() api.Daemon {
	return api.daemon
}

func (api *nodeAPI) Dag() api.Dag {
	return api.dag
}

func (api *nodeAPI) Miner() api.Miner {
	return api.miner
}

func (api *nodeAPI) Ping() api.Ping {
	return api.ping
}

func (api *nodeAPI) RetrievalClient() api.RetrievalClient {
	return api.retrievalClient
}

func (api *nodeAPI) Swarm() api.Swarm {
	return api.swarm
}
