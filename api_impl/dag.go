package api_impl

type NodeDag struct {
	api *API
}

func NewNodeDag(api *API) *NodeDag {
	return &NodeDag{api: api}
}
