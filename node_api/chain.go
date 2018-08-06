package node_api

type ChainAPI struct {
	api *API
}

func NewChainAPI(api *API) *ChainAPI {
	return &ChainAPI{api: api}
}
