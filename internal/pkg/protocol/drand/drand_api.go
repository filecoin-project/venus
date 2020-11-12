package drand

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/beacon"
	"github.com/filecoin-project/venus/internal/pkg/block"
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
