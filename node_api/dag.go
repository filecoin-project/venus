package node_api

type NodeDag struct {
	api *API
}

func NewNodeDag(api *API) *NodeDag {
	return &NodeDag{api: api}
}
