package bazaar

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
)

type Host interface {
	RegisterHandler(typ Event, hdl EventHandler) error
	Emit(ctx context.Context, evt Event, to ...peer.ID) error
}
