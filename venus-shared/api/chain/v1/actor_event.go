package v1

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type IActorEvent interface {
	// Actor events

	// GetActorEventsRaw returns all user-programmed and built-in actor events that match the given
	// filter.
	// This is a request/response API.
	// Results available from this API may be limited by the MaxFilterResults and MaxFilterHeightRange
	// configuration options and also the amount of historical data available in the node.
	//
	// This is an EXPERIMENTAL API and may be subject to change.
	GetActorEventsRaw(ctx context.Context, filter *types.ActorEventFilter) ([]*types.ActorEvent, error) //perm:read

	// SubscribeActorEventsRaw returns a long-lived stream of all user-programmed and built-in actor
	// events that match the given filter.
	// Events that match the given filter are written to the stream in real-time as they are emitted
	// from the FVM.
	// The response stream is closed when the client disconnects, when a ToHeight is specified and is
	// reached, or if there is an error while writing an event to the stream.
	// This API also allows clients to read all historical events matching the given filter before any
	// real-time events are written to the response stream if the filter specifies an earlier
	// FromHeight.
	// Results available from this API may be limited by the MaxFilterResults and MaxFilterHeightRange
	// configuration options and also the amount of historical data available in the node.
	//
	// Note: this API is only available via websocket connections.
	// This is an EXPERIMENTAL API and may be subject to change.
	SubscribeActorEventsRaw(ctx context.Context, filter *types.ActorEventFilter) (<-chan *types.ActorEvent, error) //perm:read
}
