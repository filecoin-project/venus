package node_api

type DaemonAPI struct {
	api *API
}

func NewDaemonAPI(api *API) *DaemonAPI {
	return &DaemonAPI{api: api}
}
