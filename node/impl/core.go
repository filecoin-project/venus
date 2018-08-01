// Package impl provides an implementation of the Filecoin core api.
package impl

import (
	"github.com/filecoin-project/go-filecoin/core/node"
	"github.com/filecoin-project/go-filecoin/node/iface"
)

// CoreAPI is an actual implementation of the filecoin core api interface.
type CoreAPI struct {
	node *node.Node
}

// Assert that CoreAPI fullfills the iface.CoreAPI interface.
var _ iface.CoreAPI = (*CoreAPI)(nil)

// NewCoreAPI constructs a new instance of the CoreAPI.
func NewCoreAPI(node *node.Node) iface.CoreAPI {
	api := &CoreAPI{node: node}
	return api
}

func (api *CoreAPI) Actor() iface.ActorAPI {
	return (*ActorAPI)(api)
}

func (api *CoreAPI) Address() iface.AddressAPI {
	return (*AddressAPI)(api)
}

func (api *CoreAPI) Bootstrap() iface.BootstrapAPI {
	return (*BootstrapAPI)(api)
}

func (api *CoreAPI) Chain() iface.ChainAPI {
	return (*ChainAPI)(api)
}

func (api *CoreAPI) Config() iface.ConfigAPI {
	return (*ConfigAPI)(api)
}

func (api *CoreAPI) Client() iface.ClientAPI {
	return (*ClientAPI)(api)
}

func (api *CoreAPI) Daemon() iface.DaemonAPI {
	return (*DaemonAPI)(api)
}

func (api *CoreAPI) Dag() iface.DagAPI {
	return (*DagAPI)(api)
}

func (api *CoreAPI) Id() iface.IdAPI {
	return (*IdAPI)(api)
}

func (api *CoreAPI) Init() iface.InitAPI {
	return (*InitAPI)(api)
}

func (api *CoreAPI) Log() iface.LogAPI {
	return (*LogAPI)(api)
}

func (api *CoreAPI) Message() iface.MessageAPI {
	return (*MessageAPI)(api)
}

func (api *CoreAPI) Miner() iface.MinerAPI {
	return (*MinerAPI)(api)
}

func (api *CoreAPI) Mining() iface.MiningAPI {
	return (*MiningAPI)(api)
}

func (api *CoreAPI) Mpool() iface.MpoolAPI {
	return (*MpoolAPI)(api)
}

func (api *CoreAPI) Orderbook() iface.OrderbookAPI {
	return (*OrderbookAPI)(api)
}

func (api *CoreAPI) Paych() iface.PaychAPI {
	return (*PaychAPI)(api)
}

func (api *CoreAPI) Ping() iface.PingAPI {
	return (*PingAPI)(api)
}

func (api *CoreAPI) Show() iface.ShowAPI {
	return (*ShowAPI)(api)
}

func (api *CoreAPI) Swarm() iface.SwarmAPI {
	return (*SwarmAPI)(api)
}

func (api *CoreAPI) Version() iface.VersionAPI {
	return (*VersionAPI)(api)
}

func (api *CoreAPI) Wallet() iface.WalletAPI {
	return (*WalletAPI)(api)
}

// DEPRECATED
// TODO: remove once all commands are migrated to the new api
func (api *CoreAPI) Node() *node.Node {
	return api.node
}
