package node_api

type NodeMiner struct {
	api *API
}

func NewNodeMiner(api *API) *NodeMiner {
	return &NodeMiner{api: api}
}
