package libp2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
)

type FilPeerEvtType int

const (
	AddFilPeerEvt FilPeerEvtType = iota
	RemoveFilPeerEvt
)

type FilPeerEvent struct {
	Type FilPeerEvtType
	ID   peer.ID
}

type PeerManager interface {
	AddFilecoinPeer(ctx context.Context, p peer.ID)
	GetPeerLatency(ctx context.Context, p peer.ID) (time.Duration, bool)
	SetPeerLatency(ctx context.Context, p peer.ID, latency time.Duration)
	Disconnect(ctx context.Context, p peer.ID)
}
