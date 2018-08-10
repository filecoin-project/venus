package api_impl

import (
	logging "gx/ipfs/QmcVVHfdyv15GVPk7NrxdWjh2hLVccXnoD8j2tyQShiXJb/go-log"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/node"
)

// NodeAPI is an actual implementation of the filecoin core api interface.
type NodeAPI struct {
	node   *node.Node
	logger logging.EventLogger

	actor     *NodeActor
	address   *NodeAddress
	block     *NodeBlock
	bootstrap *NodeBootstrap
	chain     *NodeChain
	config    *NodeConfig
	client    *NodeClient
	daemon    *NodeDaemon
	dag       *NodeDag
	id        *NodeID
	log       *NodeLog
	message   *NodeMessage
	miner     *NodeMiner
	mining    *NodeMining
	mpool     *NodeMpool
	orderbook *NodeOrderbook
	paych     *NodePaych
	ping      *NodePing
	swarm     *NodeSwarm
	version   *NodeVersion
}

// Assert that NodeAPI fullfills the api.API interface.
var _ api.API = (*NodeAPI)(nil)

// New constructs a new instance of the API.
func New(node *node.Node) api.API {
	api := &NodeAPI{
		node:   node,
		logger: logging.Logger("api"),
	}

	api.actor = NewNodeActor(api)
	api.address = NewNodeAddress(api)
	api.block = NewNodeBlock(api)
	api.bootstrap = NewNodeBootstrap(api)
	api.chain = NewNodeChain(api)
	api.config = NewNodeConfig(api)
	api.client = NewNodeClient(api)
	api.daemon = NewNodeDaemon(api)
	api.dag = NewNodeDag(api)
	api.id = NewNodeID(api)
	api.log = NewNodeLog(api)
	api.message = NewNodeMessage(api)
	api.miner = NewNodeMiner(api)
	api.mining = NewNodeMining(api)
	api.mpool = NewNodeMpool(api)
	api.orderbook = NewNodeOrderbook(api)
	api.paych = NewNodePaych(api)
	api.ping = NewNodePing(api)
	api.swarm = NewNodeSwarm(api)
	api.version = NewNodeVersion(api)

	return api
}

func (api *NodeAPI) Actor() api.Actor {
	return api.actor
}

func (api *NodeAPI) Address() api.Address {
	return api.address
}

func (api *NodeAPI) Block() api.Block {
	return api.block
}

func (api *NodeAPI) Bootstrap() api.Bootstrap {
	return api.bootstrap
}

func (api *NodeAPI) Chain() api.Chain {
	return api.chain
}

func (api *NodeAPI) Config() api.Config {
	return api.config
}

func (api *NodeAPI) Client() api.Client {
	return api.client
}

func (api *NodeAPI) Daemon() api.Daemon {
	return api.daemon
}

func (api *NodeAPI) Dag() api.Dag {
	return api.dag
}

func (api *NodeAPI) ID() api.ID {
	return api.id
}

func (api *NodeAPI) Log() api.Log {
	return api.log
}

func (api *NodeAPI) Message() api.Message {
	return api.message
}

func (api *NodeAPI) Miner() api.Miner {
	return api.miner
}

func (api *NodeAPI) Mining() api.Mining {
	return api.mining
}

func (api *NodeAPI) Mpool() api.Mpool {
	return api.mpool
}

func (api *NodeAPI) Orderbook() api.Orderbook {
	return api.orderbook
}

func (api *NodeAPI) Paych() api.Paych {
	return api.paych
}

func (api *NodeAPI) Ping() api.Ping {
	return api.ping
}

func (api *NodeAPI) Swarm() api.Swarm {
	return api.swarm
}

func (api *NodeAPI) Version() api.Version {
	return api.version
}
