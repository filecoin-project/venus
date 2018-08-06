package node_api

type PingAPI struct {
	api *API
}

func NewPingAPI(api *API) *PingAPI {
	return &PingAPI{api: api}
}
