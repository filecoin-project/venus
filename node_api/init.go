package node_api

type InitAPI struct {
	api *API
}

func NewInitAPI(api *API) *InitAPI {
	return &InitAPI{api: api}
}
