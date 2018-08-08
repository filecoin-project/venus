package api_impl

type NodePing struct {
	api *API
}

func NewNodePing(api *API) *NodePing {
	return &NodePing{api: api}
}
