package api_impl

type NodeMiner struct {
	api *API
}

func NewNodeMiner(api *API) *NodeMiner {
	return &NodeMiner{api: api}
}
