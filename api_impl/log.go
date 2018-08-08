package api_impl

type NodeLog struct {
	api *API
}

func NewNodeLog(api *API) *NodeLog {
	return &NodeLog{api: api}
}
