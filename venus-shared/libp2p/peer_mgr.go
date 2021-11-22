package libp2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
)

type PeerManager interface {
	AddFilecoinPeer(ctx context.Context, p peer.ID)
	GetPeerLatency(ctx context.Context, p peer.ID) (time.Duration, bool)
	SetPeerLatency(ctx context.Context, p peer.ID, latency time.Duration)
	Disconnect(ctx context.Context, p peer.ID)
}
