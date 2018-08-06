package node_api

type OrderbookAPI struct {
	api *API
}

func NewOrderbookAPI(api *API) *OrderbookAPI {
	return &OrderbookAPI{api: api}
}
