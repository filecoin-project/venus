package node_api

type ClientAPI struct {
	api *API
}

func NewClientAPI(api *API) *ClientAPI {
	return &ClientAPI{api: api}
}
