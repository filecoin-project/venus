package node_api

type NodeDaemon struct {
	api *API
}

func NewNodeDaemon(api *API) *NodeDaemon {
	return &NodeDaemon{api: api}
}
