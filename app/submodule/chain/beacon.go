package chain

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var _ v1api.IBeacon = &beaconAPI{}

type beaconAPI struct {
	chain *ChainSubmodule
}

//NewBeaconAPI create a new beacon api
func NewBeaconAPI(chain *ChainSubmodule) v1api.IBeacon {
	return &beaconAPI{chain: chain}
}

// BeaconGetEntry returns the beacon entry for the given filecoin epoch. If
// the entry has not yet been produced, the call will block until the entry
// becomes available
func (beaconAPI *beaconAPI) BeaconGetEntry(ctx context.Context, epoch abi.ChainEpoch) (*types.BeaconEntry, error) {
	return beaconAPI.chain.API().StateGetBeaconEntry(ctx, epoch)
}
