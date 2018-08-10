package impl

import (
	"context"
)

type nodeBootstrap struct {
	api *nodeAPI
}

func newNodeBootstrap(api *nodeAPI) *nodeBootstrap {
	return &nodeBootstrap{api: api}
}

func (api *nodeBootstrap) Ls(ctx context.Context) ([]string, error) {
	addrs := api.api.node.Repo.Config().Bootstrap.Addresses
	return addrs, nil
}
