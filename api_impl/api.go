package api_impl

import (
	logging "gx/ipfs/QmcVVHfdyv15GVPk7NrxdWjh2hLVccXnoD8j2tyQShiXJb/go-log"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/node"
)

// API is an actual implementation of the filecoin core api interface.
type API struct {
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
	id        *NodeId
	log       *NodeLog
	message   *NodeMessage
	miner     *NodeMiner
	mining    *NodeMining
	mpool     *NodeMpool
	orderbook *NodeOrderbook
	paych     *NodePaych
	ping      *NodePing
	show      *NodeShow
	swarm     *NodeSwarm
	version   *NodeVersion
}

// Assert that API fullfills the api.API interface.
var _ api.API = (*API)(nil)

// New constructs a new instance of the API.
func New(node *node.Node) api.API {
	api := &API{
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
	api.id = NewNodeId(api)
	api.log = NewNodeLog(api)
	api.message = NewNodeMessage(api)
	api.miner = NewNodeMiner(api)
	api.mining = NewNodeMining(api)
	api.mpool = NewNodeMpool(api)
	api.orderbook = NewNodeOrderbook(api)
	api.paych = NewNodePaych(api)
	api.ping = NewNodePing(api)
	api.show = NewNodeShow(api)
	api.swarm = NewNodeSwarm(api)
	api.version = NewNodeVersion(api)

	return api
}

func (api *API) Actor() api.Actor {
	return api.actor
}

func (api *API) Address() api.Address {
	return api.address
}

func (api *API) Block() api.Block {
	return api.block
}

func (api *API) Bootstrap() api.Bootstrap {
	return api.bootstrap
}

func (api *API) Chain() api.Chain {
	return api.chain
}

func (api *API) Config() api.Config {
	return api.config
}

func (api *API) Client() api.Client {
	return api.client
}

func (api *API) Daemon() api.Daemon {
	return api.daemon
}

func (api *API) Dag() api.Dag {
	return api.dag
}

func (api *API) Id() api.Id {
	return api.id
}

func (api *API) Log() api.Log {
	return api.log
}

func (api *API) Message() api.Message {
	return api.message
}

func (api *API) Miner() api.Miner {
	return api.miner
}

func (api *API) Mining() api.Mining {
	return api.mining
}

func (api *API) Mpool() api.Mpool {
	return api.mpool
}

func (api *API) Orderbook() api.Orderbook {
	return api.orderbook
}

func (api *API) Paych() api.Paych {
	return api.paych
}

func (api *API) Ping() api.Ping {
	return api.ping
}

func (api *API) Show() api.Show {
	return api.show
}

func (api *API) Swarm() api.Swarm {
	return api.swarm
}

func (api *API) Version() api.Version {
	return api.version
}
