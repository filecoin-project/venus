package node_api

type MiningAPI struct {
	api *API
}

func NewMiningAPI(api *API) *MiningAPI {
	return &MiningAPI{api: api}
}
