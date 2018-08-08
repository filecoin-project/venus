package api_impl

type NodeMpool struct {
	api *API
}

func NewNodeMpool(api *API) *NodeMpool {
	return &NodeMpool{api: api}
}
