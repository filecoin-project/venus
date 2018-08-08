package node_api

type NodeLog struct {
	api *API
}

func NewNodeLog(api *API) *NodeLog {
	return &NodeLog{api: api}
}
