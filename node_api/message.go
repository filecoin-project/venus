package node_api

type NodeMessage struct {
	api *API
}

func NewNodeMessage(api *API) *NodeMessage {
	return &NodeMessage{api: api}
}
