package discovery

import (
	"context"
	"sync"
	"time"

	"github.com/filecoin-project/venus/pkg/util/moresync"
	logging "github.com/ipfs/go-log/v2"
	host "github.com/libp2p/go-libp2p-core/host"
	inet "github.com/libp2p/go-libp2p-core/network"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
)

var logBootstrap = logging.Logger("net.bootstrap")

// Bootstrapper attempts to keep the p2p host connected to the filecoin network
// by keeping a minimum threshold of connections. If the threshold isn't met it
// connects to a random subset of the bootstrap peers. It does not use peer routing
// to discover new peers. To stop a Bootstrapper cancel the context passed in Start()
// or call Stop().
type Bootstrapper struct {
	// Config
	// MinPeerThreshold is the number of connections it attempts to maintain.
	MinPeerThreshold int
	// Peers to connect to if we fall below the threshold.
	bootstrapPeers []peer.AddrInfo
	// Period is the interval at which it periodically checks to see
	// if the threshold is maintained.
	Period time.Duration
	// ConnectionTimeout is how long to wait before timing out a connection attempt.
	ConnectionTimeout time.Duration

	// Dependencies
	h host.Host
	d inet.Dialer
	r routing.Routing
	// Does the work. Usually Bootstrapper.bootstrap. Argument is a slice of
	// currently-connected peers (so it won't attempt to reconnect).
	Bootstrap func([]peer.ID)

	// Bookkeeping
	/* never use `ticker` again */
	// ticker        *time.Ticker
	ctx           context.Context
	cancel        context.CancelFunc
	filecoinPeers *moresync.Latch
}

// NewBootstrapper returns a new Bootstrapper that will attempt to keep connected
// to the filecoin network by connecting to the given bootstrap peers.
func NewBootstrapper(bootstrapPeers []peer.AddrInfo, h host.Host, d inet.Dialer, r routing.Routing, minPeer int, period time.Duration) *Bootstrapper {
	b := &Bootstrapper{
		MinPeerThreshold:  minPeer,
		bootstrapPeers:    bootstrapPeers,
		Period:            period,
		ConnectionTimeout: 20 * time.Second,

		h: h,
		d: d,
		r: r,

		filecoinPeers: moresync.NewLatch(uint(minPeer)),
	}
	b.Bootstrap = b.bootstrap
	return b
}

// Start starts the Bootstrapper bootstrapping. Cancel `ctx` or call Stop() to stop it.
func (b *Bootstrapper) Start(ctx context.Context) {
	b.ctx, b.cancel = context.WithCancel(ctx)
	b.Bootstrap(nil) // boot first

	// does't need a ticker anymore
	// b.ticker = time.NewTicker(b.Period)

	/* following commented logical was replaced by `PeerManager` */
	// go func() {
	// 	defer b.ticker.Stop()
	//
	// 	for {
	// 		select {
	// 		case <-b.ctx.Done():
	// 			return
	// 		case <-b.ticker.C:
	// 			b.Bootstrap(b.d.Peers())
	// 		}
	// 	}
	// }()
}

// Stop stops the Bootstrapper.
func (b *Bootstrapper) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
}

// bootstrap does the actual work. If the number of connected peers
// has fallen below b.MinPeerThreshold it will attempt to connect to
// a random subset of its bootstrap peers.
func (b *Bootstrapper) bootstrap(currentPeers []peer.ID) {
	ctx, cancel := context.WithTimeout(b.ctx, b.ConnectionTimeout)
	var wg sync.WaitGroup
	defer func() {
		wg.Wait()
		// After connecting to bootstrap peers, bootstrap the DHT.
		// DHT Bootstrap is a persistent process so only do this once.
		cancel()
	}()

	for _, bootstrappPeer := range b.bootstrapPeers {
		pinfo := bootstrappPeer
		// Don't try to connect to an already connected peer.
		if hasPID(currentPeers, pinfo.ID) {
			continue
		}

		wg.Add(1)
		go func() {
			if err := b.h.Connect(ctx, pinfo); err != nil {
				logBootstrap.Debugf("got error trying to connect to bootstrap node %+v: %s", pinfo, err.Error())
			}
			b.h.ConnManager().TagPeer(pinfo.ID, "boot-strap", 1000)
			wg.Done()
		}()
	}
}

func hasPID(pids []peer.ID, pid peer.ID) bool {
	for _, p := range pids {
		if p == pid {
			return true
		}
	}
	return false
}
