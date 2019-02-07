package impl

import (
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/porcelain"
)

type nodeAPI struct {
	node   *node.Node
	logger logging.EventLogger

	actor           *nodeActor
	address         *nodeAddress
	block           *nodeBlock
	client          *nodeClient
	daemon          *nodeDaemon
	dag             *nodeDag
	id              *nodeID
	log             *nodeLog
	miner           *nodeMiner
	mining          *nodeMining
	mpool           *nodeMpool
	paych           *nodePaych
	ping            *nodePing
	retrievalClient *nodeRetrievalClient
	swarm           *nodeSwarm
	version         *nodeVersion
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

	api.actor = newNodeActor(api)
	api.address = newNodeAddress(api)
	api.block = newNodeBlock(api)
	api.client = newNodeClient(api)
	api.daemon = newNodeDaemon(api)
	api.dag = newNodeDag(api)
	api.id = newNodeID(api)
	api.log = newNodeLog(api)
	api.miner = newNodeMiner(api, porcelainAPI)
	api.mining = newNodeMining(api)
	api.mpool = newNodeMpool(api)
	api.paych = newNodePaych(api, porcelainAPI)
	api.ping = newNodePing(api)
	api.retrievalClient = newNodeRetrievalClient(api)
	api.swarm = newNodeSwarm(api)
	api.version = newNodeVersion(api)

	return api
}

func (api *nodeAPI) Actor() api.Actor {
	return api.actor
}

func (api *nodeAPI) Address() api.Address {
	return api.address
}

func (api *nodeAPI) Block() api.Block {
	return api.block
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

func (api *nodeAPI) ID() api.ID {
	return api.id
}

func (api *nodeAPI) Log() api.Log {
	return api.log
}

func (api *nodeAPI) Miner() api.Miner {
	return api.miner
}

func (api *nodeAPI) Mining() api.Mining {
	return api.mining
}

func (api *nodeAPI) Mpool() api.Mpool {
	return api.mpool
}

func (api *nodeAPI) Paych() api.Paych {
	return api.paych
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

func (api *nodeAPI) Version() api.Version {
	return api.version
}
