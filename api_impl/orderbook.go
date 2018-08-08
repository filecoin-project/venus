package api_impl

type NodeOrderbook struct {
	api *API
}

func NewNodeOrderbook(api *API) *NodeOrderbook {
	return &NodeOrderbook{api: api}
}
