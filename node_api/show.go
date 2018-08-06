package node_api

type ShowAPI struct {
	api *API
}

func NewShowAPI(api *API) *ShowAPI {
	return &ShowAPI{api: api}
}
