package porcelain_test

import (
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
)

type ntwkPingPlumbing struct {
	self peer.ID       // pinging this will fail immediately
	rtt  time.Duration // pinging all other ids will resolve after rtt
}
