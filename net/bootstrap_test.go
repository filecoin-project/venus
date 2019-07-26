package net

import (
	"context"
	"sync"
	"testing"
	"time"

	offroute "github.com/ipfs/go-ipfs-routing/offline"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/repo"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func panicConnect(_ context.Context, _ peer.AddrInfo) error { panic("shouldn't be called") }
func nopPeers() []peer.ID                                   { return []peer.ID{} }
func panicPeers() []peer.ID                                 { panic("shouldn't be called") }

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

func TestBootstrapperStartAndStop(t *testing.T) {
	tf.UnitTest(t)

	fakeHost := th.NewFakeHost()
	fakeDialer := &th.FakeDialer{PeersImpl: nopPeers}
	fakeRouter := offroute.NewOfflineRouter(repo.NewInMemoryRepo().Datastore(), blankValidator{})

	// Check that Start() causes Bootstrap() to be periodically called and
	// that canceling the context causes it to stop being called. Do this
	// by stubbing out Bootstrap to keep a count of the number of times it
	// is called and to cancel its context after several calls.
	b := NewBootstrapper([]peer.AddrInfo{}, fakeHost, fakeDialer, fakeRouter, 0, 200*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// protects callCount
	var lk sync.Mutex
	callCount := 0
	b.Bootstrap = func([]peer.ID) {
		lk.Lock()
		defer lk.Unlock()
		callCount++
		if callCount == 3 {

			// If b.Period is configured to be a too small, b.ticker will tick
			// again before the context's done-channel sees a value. This
			// results in a callCount of 4 instead of 3.
			cancel()
		}
	}

	b.Start(ctx)
	time.Sleep(1000 * time.Millisecond)

	lk.Lock()
	defer lk.Unlock()
	assert.Equal(t, 3, callCount)
}

func TestBootstrapperBootstrap(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Doesn't connect if already have enough peers", func(t *testing.T) {
		fakeHost := &th.FakeHost{ConnectImpl: panicConnect}
		fakeDialer := &th.FakeDialer{PeersImpl: panicPeers}
		fakeRouter := offroute.NewOfflineRouter(repo.NewInMemoryRepo().Datastore(), blankValidator{})
		ctx := context.Background()

		b := NewBootstrapper([]peer.AddrInfo{}, fakeHost, fakeDialer, fakeRouter, 1, time.Minute)
		currentPeers := []peer.ID{th.RequireRandomPeerID(t)} // Have 1
		b.ctx = ctx
		assert.NotPanics(t, func() { b.bootstrap(currentPeers) })
	})

	var lk sync.Mutex
	var connectCount int
	countingConnect := func(context.Context, peer.AddrInfo) error {
		lk.Lock()
		defer lk.Unlock()
		connectCount++
		return nil
	}

	t.Run("Connects if don't have enough peers", func(t *testing.T) {
		fakeHost := &th.FakeHost{ConnectImpl: countingConnect}
		lk.Lock()
		connectCount = 0
		lk.Unlock()
		fakeDialer := &th.FakeDialer{PeersImpl: panicPeers}
		fakeRouter := offroute.NewOfflineRouter(repo.NewInMemoryRepo().Datastore(), blankValidator{})

		bootstrapPeers := []peer.AddrInfo{
			{ID: th.RequireRandomPeerID(t)},
			{ID: th.RequireRandomPeerID(t)},
		}
		b := NewBootstrapper(bootstrapPeers, fakeHost, fakeDialer, fakeRouter, 3, time.Minute)
		b.ctx = context.Background()
		currentPeers := []peer.ID{th.RequireRandomPeerID(t)} // Have 1
		b.bootstrap(currentPeers)
		time.Sleep(20 * time.Millisecond)
		lk.Lock()
		assert.Equal(t, 2, connectCount)
		lk.Unlock()
	})

	t.Run("Doesn't try to connect to an already connected peer", func(t *testing.T) {
		fakeHost := &th.FakeHost{ConnectImpl: countingConnect}
		lk.Lock()
		connectCount = 0
		lk.Unlock()
		fakeDialer := &th.FakeDialer{PeersImpl: panicPeers}
		fakeRouter := offroute.NewOfflineRouter(repo.NewInMemoryRepo().Datastore(), blankValidator{})

		connectedPeerID := th.RequireRandomPeerID(t)
		bootstrapPeers := []peer.AddrInfo{
			{ID: connectedPeerID},
		}

		b := NewBootstrapper(bootstrapPeers, fakeHost, fakeDialer, fakeRouter, 2, time.Minute) // Need 2 bootstrap peers.
		b.ctx = context.Background()
		currentPeers := []peer.ID{connectedPeerID} // Have 1, which is the bootstrap peer.
		b.bootstrap(currentPeers)
		time.Sleep(20 * time.Millisecond)
		lk.Lock()
		assert.Equal(t, 0, connectCount)
		lk.Unlock()
	})
}
