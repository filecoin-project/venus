package api_impl

type NodeShow struct {
	api *NodeAPI
}

func NewNodeShow(api *NodeAPI) *NodeShow {
	return &NodeShow{api: api}
}
