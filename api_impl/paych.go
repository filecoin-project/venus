package api_impl

type NodePaych struct {
	api *API
}

func NewNodePaych(api *API) *NodePaych {
	return &NodePaych{api: api}
}
