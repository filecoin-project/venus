package actorevent

import (
	"context"
	"errors"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var ErrActorEventModuleDisabled = errors.New("module disabled, enable with Fevm.EnableActorEventsAPI")

type ActorEventDummy struct{} // nolint

func (a *ActorEventDummy) GetActorEventsRaw(ctx context.Context, filter *types.ActorEventFilter) ([]*types.ActorEvent, error) {
	return nil, ErrActorEventModuleDisabled
}

func (a *ActorEventDummy) SubscribeActorEventsRaw(ctx context.Context, filter *types.ActorEventFilter) (<-chan *types.ActorEvent, error) {
	return nil, ErrActorEventModuleDisabled
}

var _ v1api.IActorEvent = &ActorEventDummy{}
