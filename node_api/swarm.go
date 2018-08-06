package node_api

type SwarmAPI struct {
	api *API
}

func NewSwarmAPI(api *API) *SwarmAPI {
	return &SwarmAPI{api: api}
}
