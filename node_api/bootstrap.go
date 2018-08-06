package node_api

import (
	"context"
)

type BootstrapAPI struct {
	api *API
}

func NewBootstrapAPI(api *API) *BootstrapAPI {
	return &BootstrapAPI{api: api}
}

func (api *BootstrapAPI) Ls(ctx context.Context) ([]string, error) {
	addrs := api.api.node.Repo.Config().Bootstrap.Addresses
	return addrs, nil
}
