package filnet

import (
	"context"
	"math/rand"
	"sync"
	"time"

	host "gx/ipfs/QmPMtD39NN63AEUNghk1LFQcTLcCmYL8MtRzdv8BRUsC4Z/go-libp2p-host"
	inet "gx/ipfs/QmQSbtGXCyNrj34LWL8EgXyNNYDZ8r3SwQcpW5pPxVhLnM/go-libp2p-net"
	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	pstore "gx/ipfs/QmeKD8YT7887Xu6Z86iZmpYNxrLogJexqxEugSmaf14k64/go-libp2p-peerstore"
)

var log = logging.Logger("bootstrap")

// Bootstrapper attempts to keep the p2p host connected to the filecoin network
// by keeping a minimum threshold of connections. If the threshold isn't met it
// connects to a random subset of the bootstrap peers. It does not use peer routing
// to discover new peers. To stop a Bootstrapper cancel the context passed in Start()
// or call Stop().
//
// Code loosely modeled on go-ipfs/core/bootstrap.go, take a look there for inspiration
// if you're adding new features.
//
// TODO discover new peers
type Bootstrapper struct {
	// Config
	// MinPeerThreshold is the number of connections it attempts to maintain.
	MinPeerThreshold int
	// Peers to connect to if we fall below the threshold.
	bootstrapPeers []pstore.PeerInfo
	// Period is the interval at which it periodically checks to see
	// if the threshold is maintained.
	Period time.Duration
	// ConnectionTimeout is how long to wait before timing out a connection attempt.
	ConnectionTimeout time.Duration

	// Dependencies
	h host.Host
	d inet.Dialer
	// Does the work. Usually Bootstrapper.bootstrap. Argument is a slice of
	// currently-connected peers (so it won't attempt to reconnect).
	Bootstrap func([]peer.ID)

	// Bookkeeping
	ticker *time.Ticker
	ctx    context.Context
	cancel context.CancelFunc
}

// NewBootstrapper returns a new Bootstrapper that will attempt to keep connected
// to the filecoin network by connecting to the given bootstrap peers.
func NewBootstrapper(bootstrapPeers []pstore.PeerInfo, h host.Host, d inet.Dialer, minPeer int, period time.Duration) *Bootstrapper {
	b := &Bootstrapper{
		MinPeerThreshold:  minPeer,
		bootstrapPeers:    bootstrapPeers,
		Period:            period,
		ConnectionTimeout: 20 * time.Second,

		h: h,
		d: d,
	}
	b.Bootstrap = b.bootstrap
	return b
}

// Start starts the Bootstrapper bootstrapping. Cancel `ctx` or call Stop() to stop it.
func (b *Bootstrapper) Start(ctx context.Context) {
	b.ctx, b.cancel = context.WithCancel(ctx)
	b.ticker = time.NewTicker(b.Period)

	go func() {
		defer b.ticker.Stop()

		for {
			select {
			case <-b.ctx.Done():
				return
			case <-b.ticker.C:
				b.Bootstrap(b.d.Peers())
			}
		}
	}()
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
	peersNeeded := b.MinPeerThreshold - len(currentPeers)
	if peersNeeded < 1 {
		return
	}

	ctx, cancel := context.WithTimeout(b.ctx, b.ConnectionTimeout)
	var wg sync.WaitGroup
	defer func() {
		wg.Wait()
		cancel()
	}()

	peersAttempted := 0
	for _, i := range rand.Perm(len(b.bootstrapPeers)) {
		pinfo := b.bootstrapPeers[i]
		// Don't try to connect to an already connected peer.
		if hasPID(currentPeers, pinfo.ID) {
			continue
		}

		wg.Add(1)
		go func() {
			if err := b.h.Connect(ctx, pinfo); err != nil {
				log.Warningf("Got error trying to connect to bootstrap node %+v: %s", pinfo, err.Error())
			}
			wg.Done()
		}()
		peersAttempted++
		if peersAttempted == peersNeeded {
			return
		}
	}
	log.Warningf("Not enough bootstrap nodes to maintain %d connections (current connections: %d)", b.MinPeerThreshold, len(currentPeers))
}

func hasPID(pids []peer.ID, pid peer.ID) bool {
	for _, p := range pids {
		if p == pid {
			return true
		}
	}
	return false
}
