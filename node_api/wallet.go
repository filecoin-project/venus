package node_api

type WalletAPI struct {
	api *API
}

func NewWalletAPI(api *API) *WalletAPI {
	return &WalletAPI{api: api}
}
