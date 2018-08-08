package node_api

type NodePing struct {
	api *API
}

func NewNodePing(api *API) *NodePing {
	return &NodePing{api: api}
}
