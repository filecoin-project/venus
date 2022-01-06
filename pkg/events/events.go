package events

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var log = logging.Logger("events")

// HeightHandler `curH`-`ts.Height` = `confidence`
type (
	HeightHandler func(ctx context.Context, ts *types.TipSet, curH abi.ChainEpoch) error
	RevertHandler func(ctx context.Context, ts *types.TipSet) error
)

type Events struct {
	*observer
	*heightEvents
	*hcEvents
}

func NewEventsWithConfidence(ctx context.Context, api IEvent, gcConfidence abi.ChainEpoch) (*Events, error) {
	cache := newCache(api, gcConfidence)

	ob := newObserver(cache, gcConfidence)
	if err := ob.start(ctx); err != nil {
		return nil, err
	}

	he := newHeightEvents(cache, ob, gcConfidence)
	headChange := newHCEvents(cache, ob)

	return &Events{ob, he, headChange}, nil
}

func NewEvents(ctx context.Context, api IEvent) (*Events, error) {
	gcConfidence := 2 * constants.ForkLengthThreshold
	return NewEventsWithConfidence(ctx, api, gcConfidence)
}
