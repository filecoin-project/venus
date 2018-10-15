package impl

import (
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/node"
)

type nodeAPI struct {
	node   *node.Node
	logger logging.EventLogger

	actor           *nodeActor
	address         *nodeAddress
	block           *nodeBlock
	bootstrap       *nodeBootstrap
	chain           *nodeChain
	config          *nodeConfig
	client          *nodeClient
	daemon          *nodeDaemon
	dag             *nodeDag
	id              *nodeID
	log             *nodeLog
	message         *nodeMessage
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

	api.actor = newNodeActor(api)
	api.address = newNodeAddress(api)
	api.block = newNodeBlock(api)
	api.bootstrap = newNodeBootstrap(api)
	api.chain = newNodeChain(api)
	api.config = newNodeConfig(api)
	api.client = newNodeClient(api)
	api.daemon = newNodeDaemon(api)
	api.dag = newNodeDag(api)
	api.id = newNodeID(api)
	api.log = newNodeLog(api)
	api.message = newNodeMessage(api)
	api.miner = newNodeMiner(api)
	api.mining = newNodeMining(api)
	api.mpool = newNodeMpool(api)
	api.paych = newNodePaych(api)
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

func (api *nodeAPI) Bootstrap() api.Bootstrap {
	return api.bootstrap
}

func (api *nodeAPI) Chain() api.Chain {
	return api.chain
}

func (api *nodeAPI) Config() api.Config {
	return api.config
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

func (api *nodeAPI) Message() api.Message {
	return api.message
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
