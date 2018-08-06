package node_api

type VersionAPI struct {
	api *API
}

func NewVersionAPI(api *API) *VersionAPI {
	return &VersionAPI{api: api}
}
