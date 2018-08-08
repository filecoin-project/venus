package node_api

type NodePaych struct {
	api *API
}

func NewNodePaych(api *API) *NodePaych {
	return &NodePaych{api: api}
}
