package node_api

type NodeId struct {
	api *API
}

func NewNodeId(api *API) *NodeId {
	return &NodeId{api: api}
}
