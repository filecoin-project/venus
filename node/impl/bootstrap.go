package impl

import (
	"context"
)

type BootstrapAPI struct {
	api *CoreAPI
}

func (api *BootstrapAPI) Ls(ctx context.Context) ([]string, error) {
	addrs := api.api.node.Repo.Config().Bootstrap.Addresses
	return addrs, nil
}
