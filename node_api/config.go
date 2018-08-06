package node_api

type ConfigAPI struct {
	api *API
}

func NewConfigAPI(api *API) *ConfigAPI {
	return &ConfigAPI{api: api}
}
