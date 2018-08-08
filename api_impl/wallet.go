package api_impl

type NodeWallet struct {
	api *API
}

func NewNodeWallet(api *API) *NodeWallet {
	return &NodeWallet{api: api}
}
