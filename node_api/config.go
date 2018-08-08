package node_api

type NodeConfig struct {
	api *API
}

func NewNodeConfig(api *API) *NodeConfig {
	return &NodeConfig{api: api}
}
