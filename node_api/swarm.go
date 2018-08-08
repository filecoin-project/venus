package node_api

type NodeSwarm struct {
	api *API
}

func NewNodeSwarm(api *API) *NodeSwarm {
	return &NodeSwarm{api: api}
}
