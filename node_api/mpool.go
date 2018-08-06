package node_api

type MpoolAPI struct {
	api *API
}

func NewMpoolAPI(api *API) *MpoolAPI {
	return &MpoolAPI{api: api}
}
