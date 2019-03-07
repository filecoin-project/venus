package net

import (
	"context"
	"sync"
	"testing"
	"time"

	pstore "gx/ipfs/QmRhFARzTHcFh8wUxwN5KvyTGq73FLC65EfFAhz8Ng7aGb/go-libp2p-peerstore"
	peer "gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	offroute "gx/ipfs/QmcjqHcsk8E1Gd8RbuaUawWC7ogDtaVcdjLvZF8ysCCiPn/go-ipfs-routing/offline"

	"github.com/filecoin-project/go-filecoin/repo"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func nopConnect(context.Context, pstore.PeerInfo) error   { return nil }
func panicConnect(context.Context, pstore.PeerInfo) error { panic("shouldn't be called") }
func nopPeers() []peer.ID                                 { return []peer.ID{} }
func panicPeers() []peer.ID                               { panic("shouldn't be called") }

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

func TestBootstrapperStartAndStop(t *testing.T) {
	assert := assert.New(t)
	fakeHost := &fakeHost{ConnectImpl: nopConnect}
	fakeDialer := &fakeDialer{PeersImpl: nopPeers}
	fakeRouter := offroute.NewOfflineRouter(repo.NewInMemoryRepo().Datastore(), blankValidator{})

	// Check that Start() causes Bootstrap() to be periodically called and
	// that canceling the context causes it to stop being called. Do this
	// by stubbing out Bootstrap to keep a count of the number of times it
	// is called and to cancel its context after several calls.
	b := NewBootstrapper([]pstore.PeerInfo{}, fakeHost, fakeDialer, fakeRouter, 0, 200*time.Millisecond)
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
	assert.Equal(3, callCount)
}

func TestBootstrapperBootstrap(t *testing.T) {
	t.Run("Doesn't connect if already have enough peers", func(t *testing.T) {
		assert := assert.New(t)
		fakeHost := &fakeHost{ConnectImpl: panicConnect}
		fakeDialer := &fakeDialer{PeersImpl: panicPeers}
		fakeRouter := offroute.NewOfflineRouter(repo.NewInMemoryRepo().Datastore(), blankValidator{})
		ctx := context.Background()

		b := NewBootstrapper([]pstore.PeerInfo{}, fakeHost, fakeDialer, fakeRouter, 1, time.Minute)
		currentPeers := []peer.ID{requireRandPeerID(t)} // Have 1
		b.ctx = ctx
		assert.NotPanics(func() { b.bootstrap(currentPeers) })
	})

	var lk sync.Mutex
	var connectCount int
	countingConnect := func(context.Context, pstore.PeerInfo) error {
		lk.Lock()
		defer lk.Unlock()
		connectCount++
		return nil
	}

	t.Run("Connects if don't have enough peers", func(t *testing.T) {
		assert := assert.New(t)
		fakeHost := &fakeHost{ConnectImpl: countingConnect}
		lk.Lock()
		connectCount = 0
		lk.Unlock()
		fakeDialer := &fakeDialer{PeersImpl: panicPeers}
		fakeRouter := offroute.NewOfflineRouter(repo.NewInMemoryRepo().Datastore(), blankValidator{})

		bootstrapPeers := []pstore.PeerInfo{
			{ID: requireRandPeerID(t)},
			{ID: requireRandPeerID(t)},
		}
		b := NewBootstrapper(bootstrapPeers, fakeHost, fakeDialer, fakeRouter, 3, time.Minute)
		b.ctx = context.Background()
		currentPeers := []peer.ID{requireRandPeerID(t)} // Have 1
		b.bootstrap(currentPeers)
		time.Sleep(20 * time.Millisecond)
		lk.Lock()
		assert.Equal(2, connectCount)
		lk.Unlock()
	})

	t.Run("Doesn't try to connect to an already connected peer", func(t *testing.T) {
		assert := assert.New(t)
		fakeHost := &fakeHost{ConnectImpl: countingConnect}
		lk.Lock()
		connectCount = 0
		lk.Unlock()
		fakeDialer := &fakeDialer{PeersImpl: panicPeers}
		fakeRouter := offroute.NewOfflineRouter(repo.NewInMemoryRepo().Datastore(), blankValidator{})

		connectedPeerID := requireRandPeerID(t)
		bootstrapPeers := []pstore.PeerInfo{
			{ID: connectedPeerID},
		}

		b := NewBootstrapper(bootstrapPeers, fakeHost, fakeDialer, fakeRouter, 2, time.Minute) // Need 2 bootstrap peers.
		b.ctx = context.Background()
		currentPeers := []peer.ID{connectedPeerID} // Have 1, which is the bootstrap peer.
		b.bootstrap(currentPeers)
		time.Sleep(20 * time.Millisecond)
		lk.Lock()
		assert.Equal(0, connectCount)
		lk.Unlock()
	})
}
