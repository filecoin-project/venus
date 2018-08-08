package api_impl

type NodeMining struct {
	api *API
}

func NewNodeMining(api *API) *NodeMining {
	return &NodeMining{api: api}
}
