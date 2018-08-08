package node_api

type NodeMpool struct {
	api *API
}

func NewNodeMpool(api *API) *NodeMpool {
	return &NodeMpool{api: api}
}
