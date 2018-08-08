package api_impl

type NodeDaemon struct {
	api *API
}

func NewNodeDaemon(api *API) *NodeDaemon {
	return &NodeDaemon{api: api}
}
