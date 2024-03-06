package actorevent

import (
	"context"
	"errors"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var ErrActorEventModuleDisabled = errors.New("module disabled, enable with Fevm.EnableActorEventsAPI")

type ActorEventDummy struct{}

func (a *ActorEventDummy) GetActorEvents(ctx context.Context, filter *types.ActorEventFilter) ([]*types.ActorEvent, error) {
	return nil, ErrActorEventModuleDisabled
}

func (a *ActorEventDummy) SubscribeActorEvents(ctx context.Context, filter *types.ActorEventFilter) (<-chan *types.ActorEvent, error) {
	return nil, ErrActorEventModuleDisabled
}

var _ v1api.IActorEvent = &ActorEventDummy{}
