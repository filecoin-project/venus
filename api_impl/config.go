package api_impl

type NodeConfig struct {
	api *API
}

func NewNodeConfig(api *API) *NodeConfig {
	return &NodeConfig{api: api}
}
