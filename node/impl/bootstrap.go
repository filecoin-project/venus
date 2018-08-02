package impl

import (
	"context"
)

type BootstrapAPI struct {
	api *CoreAPI
}

func NewBootstrapAPI(api *CoreAPI) *BootstrapAPI {
	return &BootstrapAPI{api: api}
}

func (api *BootstrapAPI) Ls(ctx context.Context) ([]string, error) {
	addrs := api.api.node.Repo.Config().Bootstrap.Addresses
	return addrs, nil
}
