package node_api

type PaychAPI struct {
	api *API
}

func NewPaychAPI(api *API) *PaychAPI {
	return &PaychAPI{api: api}
}
