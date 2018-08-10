package impl

import (
	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/flags"
)

type NodeVersion struct {
	api *NodeAPI
}

func NewNodeVersion(api *NodeAPI) *NodeVersion {
	return &NodeVersion{api: api}
}

// Full, returns all version information that is available.
func (a *NodeVersion) Full() (*api.VersionInfo, error) {
	return &api.VersionInfo{
		Commit: flags.Commit,
	}, nil
}
