package api_impl

import (
	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/flags"
)

type NodeVersion struct {
	api *API
}

func NewNodeVersion(api *API) *NodeVersion {
	return &NodeVersion{api: api}
}

// Full, returns all version information that is available.
func (a *NodeVersion) Full() (*api.VersionInfo, error) {
	return &api.VersionInfo{
		Commit: flags.Commit,
	}, nil
}
