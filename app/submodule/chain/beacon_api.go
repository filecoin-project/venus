package chain

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/block"
)

type IBeacon interface {
	BeaconGetEntry(ctx context.Context, epoch abi.ChainEpoch) (*block.BeaconEntry, error)
}
type BeaconAPI struct {
	chain *ChainSubmodule
}

func NewBeaconAPI(chain *ChainSubmodule) BeaconAPI {
	return BeaconAPI{chain: chain}
}

func (beaconAPI *BeaconAPI) BeaconGetEntry(ctx context.Context, epoch abi.ChainEpoch) (*block.BeaconEntry, error) {
	b := beaconAPI.chain.Drand.BeaconForEpoch(epoch)
	rr := b.MaxBeaconRoundForEpoch(epoch)
	e := b.Entry(ctx, rr)

	select {
	case be, ok := <-e:
		if !ok {
			return nil, fmt.Errorf("beacon get returned no value")
		}
		if be.Err != nil {
			return nil, be.Err
		}
		return &be.Entry, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
