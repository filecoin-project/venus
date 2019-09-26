package net

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/clock"
	"github.com/filecoin-project/go-filecoin/types"
)

// GraphExchange is an interface wrapper to Graphsync so it can be stubbed in
// unit testing
type GraphExchange interface {
	Request(ctx context.Context, p peer.ID, root ipld.Link, selector ipld.Node) (<-chan graphsync.ResponseProgress, <-chan error)
}

// GraphsyncSession is an interface for graphsync queries
type GraphsyncSession interface {
	Request(ctx context.Context, roots []cid.Cid, selector ipld.Node, tryAllPeers bool) error
}

// GraphsyncSessionExchange is an interface which supports
// making sessions for graphsync queuries.
type GraphsyncSessionExchange interface {
	NewSession(ctx context.Context, originatingPeer peer.ID) (GraphsyncSession, error)
}

// RequestPeerTracker is an interface that returns information about connected peers
// and their chains (an interface version of PeerTracker)
type RequestPeerTracker interface {
	List() []*types.ChainInfo
	Self() peer.ID
}

type DefaultGraphsyncSessionExchange struct {
	exchange    GraphExchange
	systemClock clock.Clock
	peerTracker RequestPeerTracker
}

// MakeDefaultGraphsyncSessionExchange makes a GraphsyncSessionExchange that
// returns DefaultGraphsyncSessions connected to the given peer tracker
func MakeDefaultGraphsyncSessionExchange(exchange GraphExchange, systemClock clock.Clock, peerTracker RequestPeerTracker) *DefaultGraphsyncSessionExchange {
	return &DefaultGraphsyncSessionExchange{
		exchange:    exchange,
		systemClock: systemClock,
		peerTracker: peerTracker,
	}
}

func (gse *DefaultGraphsyncSessionExchange) NewSession(ctx context.Context, originatingPeer peer.ID) (GraphsyncSession, error) {
	fetchFromSelf := originatingPeer == gse.peerTracker.Self()
	gss := &DefaultGraphsyncSession{
		exchange:    gse.exchange,
		systemClock: gse.systemClock,
		peerTracker: gse.peerTracker,
		triedPeers:  make(map[peer.ID]struct{}),
	}

	// If the new cid triggering this request came from ourselves then
	// the first peer to request from should be ourselves.
	if fetchFromSelf {
		gss.triedPeers[gse.peerTracker.Self()] = struct{}{}
		gss.currentPeer = gse.peerTracker.Self()
		return gss, nil
	}

	// Get a peer ID from the peer tracker
	err := gss.FindNextPeers()
	if err != nil {
		return nil, err
	}
	return gss, nil
}

// DefaultGraphsyncSession is the default implementation of a graphsync session
// It simply iterates through peers in the peer tracker, one at a time
// and handles timeouts and retries
type DefaultGraphsyncSession struct {
	exchange    GraphExchange
	systemClock clock.Clock
	peerTracker RequestPeerTracker
	currentPeer peer.ID
	triedPeers  map[peer.ID]struct{}
}

// CurrentPeers returns one or more peers to make the next requests to
func (gss *DefaultGraphsyncSession) CurrentPeers() []peer.ID {
	return []peer.ID{gss.currentPeer}
}

// FindNextPeers tells the peer finder that the current peers did not successfully
// complete a request and it should look for more
func (gss *DefaultGraphsyncSession) FindNextPeers() error {
	chains := gss.peerTracker.List()
	for _, chain := range chains {
		if _, tried := gss.triedPeers[chain.Peer]; !tried {
			gss.triedPeers[chain.Peer] = struct{}{}
			gss.currentPeer = chain.Peer
			return nil
		}
	}
	return fmt.Errorf("Unable to find any untried peers")
}

func (gss *DefaultGraphsyncSession) Request(ctx context.Context, roots []cid.Cid, selector ipld.Node, tryAllPeers bool) error {
	remainingRoots := roots
	for {
		peers := gss.CurrentPeers()
		remainingRoots = gss.fetchInParallel(ctx, remainingRoots, selector, peers)
		if len(remainingRoots) == 0 {
			return nil
		}
		err := gss.FindNextPeers()
		if err != nil {
			return errors.Wrapf(err, "makingquery for cids: %s", roots)
		}
		if !tryAllPeers {
			return nil
		}
	}
}

// fetchInParallel performs the given fetch operation specified by the given roots and selector to
// multiple peers at once
func (gss *DefaultGraphsyncSession) fetchInParallel(ctx context.Context, roots []cid.Cid, selector ipld.Node, targetPeers []peer.ID) []cid.Cid {
	var wg sync.WaitGroup
	parallelCtx, parallelCancel := context.WithCancel(ctx)
	var resultLk sync.RWMutex
	globalIncomplete := make([]cid.Cid, 0, len(roots))
	for _, c := range roots {
		globalIncomplete = append(globalIncomplete, c)
	}
	defer parallelCancel()
	for _, p := range targetPeers {
		wg.Add(1)
		go func(p peer.ID) {
			defer wg.Done()
			incomplete, err := gss.fetchBlocks(parallelCtx, roots, selector, p)
			fmt.Println(incomplete)
			fmt.Println(err)
			select {
			case <-parallelCtx.Done():
			default:
			}
			resultLk.Lock()
			newGlobalIncomplete := make([]cid.Cid, 0, len(globalIncomplete))
			for _, c1 := range globalIncomplete {
				for _, c2 := range incomplete {
					if c1.Equals(c2) {
						newGlobalIncomplete = append(newGlobalIncomplete, c1)
						break
					}
				}
			}
			globalIncomplete = newGlobalIncomplete
			done := len(globalIncomplete) == 0
			resultLk.Unlock()
			if done {
				parallelCancel()
			}
			if err != nil {
				// A likely case is the peer doesn't have the tipset. When graphsync provides
				// this status we should quiet this log.
				logGraphsyncFetcher.Infof("request failed: %s", err)
			}
		}(p)
	}
	wg.Wait()
	return globalIncomplete
}

// fetchBlocks requests a single set of cids as individual blocks, fetching
// non-recursively
func (gss *DefaultGraphsyncSession) fetchBlocks(ctx context.Context, cids []cid.Cid, selector ipld.Node, targetPeer peer.ID) ([]cid.Cid, error) {
	var wg sync.WaitGroup
	// Any of the multiple parallel requests might fail. Wait for all of them to complete, then
	// return any error (in this case, the first one to be received).
	var resultLk sync.RWMutex
	var anyError error
	var incomplete []cid.Cid
	for _, c := range cids {
		requestCtx, requestCancel := context.WithCancel(ctx)
		defer requestCancel()
		requestChan, errChan := gss.exchange.Request(requestCtx, targetPeer, cidlink.Link{Cid: c}, selector)
		wg.Add(1)
		go func(requestChan <-chan graphsync.ResponseProgress, errChan <-chan error, cancelFunc func(), c cid.Cid) {
			defer wg.Done()
			err := gss.consumeResponse(requestChan, errChan, cancelFunc)
			if err != nil {
				resultLk.Lock()
				incomplete = append(incomplete, c)
				anyError = err
				resultLk.Unlock()
			}
		}(requestChan, errChan, requestCancel, c)
	}
	wg.Wait()
	return incomplete, anyError
}

func (gss *DefaultGraphsyncSession) consumeResponse(requestChan <-chan graphsync.ResponseProgress, errChan <-chan error, cancelFunc func()) error {
	timer := gss.systemClock.NewTimer(progressTimeout)
	var anyError error
	for errChan != nil || requestChan != nil {
		select {
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
			}
			anyError = err
			timer.Reset(progressTimeout)
		case _, ok := <-requestChan:
			if !ok {
				requestChan = nil
			}
			timer.Reset(progressTimeout)
		case <-timer.Chan():
			cancelFunc()
		}
	}
	return anyError
}
