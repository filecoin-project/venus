package impl

import (
	"context"
)

type NodeBootstrap struct {
	api *NodeAPI
}

func NewNodeBootstrap(api *NodeAPI) *NodeBootstrap {
	return &NodeBootstrap{api: api}
}

func (api *NodeBootstrap) Ls(ctx context.Context) ([]string, error) {
	addrs := api.api.node.Repo.Config().Bootstrap.Addresses
	return addrs, nil
}
