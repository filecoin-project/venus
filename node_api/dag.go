package node_api

type DagAPI struct {
	api *API
}

func NewDagAPI(api *API) *DagAPI {
	return &DagAPI{api: api}
}
