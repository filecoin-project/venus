package node_api

type NodeOrderbook struct {
	api *API
}

func NewNodeOrderbook(api *API) *NodeOrderbook {
	return &NodeOrderbook{api: api}
}
