package api_impl

type NodeInit struct {
	api *API
}

func NewNodeInit(api *API) *NodeInit {
	return &NodeInit{api: api}
}
