package drand

import (
	"context"
	"github.com/filecoin-project/go-filecoin/internal/pkg/beacon"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-state-types/abi"
	xerrors "github.com/pkg/errors"
)

type Config interface {
	ConfigSet(dottedPath string, paramJSON string) error
}

type API struct {
	drand  beacon.Schedule
	config Config
}

// New creates a new API
func New(drand beacon.Schedule, config Config) *API {
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
func (api *API) GetEntry(ctx context.Context, height abi.ChainEpoch, round uint64) (*block.BeaconEntry, error) {
	rch := api.drand.BeaconForEpoch(height).Entry(ctx, round)
	select {
	case resp := <-rch:
		if resp.Err != nil {
			return nil, xerrors.Errorf("beacon entry request returned error: %s", resp.Err)
		}
		return &resp.Entry, nil
	case <-ctx.Done():
		return nil, xerrors.Errorf("context timed out waiting on beacon entry to come back for round %d: %s", round, ctx.Err())
	}

}

// VerifyEntry verifies that child is a valid entry if its parent is.
func (api *API) VerifyEntry(parent, child *block.BeaconEntry, height abi.ChainEpoch) bool {
	return api.drand.BeaconForEpoch(height).VerifyEntry(*parent, *child) != nil
}
