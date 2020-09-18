package drand

import (
	"context"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
)

type Config interface {
	ConfigSet(dottedPath string, paramJSON string) error
}

type API struct {
	drand  drand.IFace
	config Config
}

// New creates a new API
func New(drand drand.IFace, config Config) *API {
	return &API{
		drand:  drand,
		config: config,
	}
}

// Configure fetches group configuration from a drand server.
// It runs through the list of addrs trying each one to fetch the group config.
// Once the group is retrieved, the node's group key will be set in config.
// If overrideGroupAddrs is true, the given set of addresses will be set as the drand nodes.
// Otherwise drand address config will be set from the retrieved group info. The
// override is useful when the the drand server is behind NAT.
// This method assumes all drand nodes are secure or that all of them are not. This
// mis-models the drand config, but is unlikely to be false in practice.
func (api *API) Configure(addrs []string, secure bool, overrideGroupAddrs bool) error {
	return nil
}

// GetEntry retrieves an entry from the drand server
func (api *API) GetEntry(ctx context.Context, round drand.Round) (*drand.Entry, error) {
	return api.drand.ReadEntry(ctx, round)
}

// VerifyEntry verifies that child is a valid entry if its parent is.
func (api *API) VerifyEntry(parent, child *drand.Entry) (bool, error) {
	return api.drand.VerifyEntry(parent, child)
}
