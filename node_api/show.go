package node_api

type NodeShow struct {
	api *API
}

func NewNodeShow(api *API) *NodeShow {
	return &NodeShow{api: api}
}
