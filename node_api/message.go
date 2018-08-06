package node_api

type MessageAPI struct {
	api *API
}

func NewMessageAPI(api *API) *MessageAPI {
	return &MessageAPI{api: api}
}
