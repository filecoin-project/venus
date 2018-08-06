package node_api

type IdAPI struct {
	api *API
}

func NewIdAPI(api *API) *IdAPI {
	return &IdAPI{api: api}
}
