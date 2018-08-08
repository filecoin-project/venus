package node_api

type NodeVersion struct {
	api *API
}

func NewNodeVersion(api *API) *NodeVersion {
	return &NodeVersion{api: api}
}
