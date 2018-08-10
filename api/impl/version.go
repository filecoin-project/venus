package impl

import (
	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/flags"
)

type nodeVersion struct {
	api *nodeAPI
}

func newNodeVersion(api *nodeAPI) *nodeVersion {
	return &nodeVersion{api: api}
}

// Full, returns all version information that is available.
func (a *nodeVersion) Full() (*api.VersionInfo, error) {
	return &api.VersionInfo{
		Commit: flags.Commit,
	}, nil
}
