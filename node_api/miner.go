package node_api

type MinerAPI struct {
	api *API
}

func NewMinerAPI(api *API) *MinerAPI {
	return &MinerAPI{api: api}
}
