package node_api

type LogAPI struct {
	api *API
}

func NewLogAPI(api *API) *LogAPI {
	return &LogAPI{api: api}
}
