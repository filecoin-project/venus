package net

import (
	"context"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	"github.com/ipld/go-ipld-prime"
	ipldfree "github.com/ipld/go-ipld-prime/impl/free"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	selectorbuilder "github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
)

var logGraphsyncFetcher = logging.Logger("net.graphsync_fetcher")

// interface conformance check
var _ Fetcher = (*GraphSyncFetcher)(nil)

// GraphExchange is an interface wrapper to Graphsync so it can be stubbed in
// unit testing
type GraphExchange interface {
	Request(ctx context.Context, p peer.ID, root ipld.Link, selector ipld.Node) (<-chan graphsync.ResponseProgress, <-chan error)
}

type graphsyncFallbackPeerTracker interface {
	List() []*types.ChainInfo
}

// GraphSyncFetcher is used to fetch data over the network.  It is implemented
// using a Graphsync exchange to fetch tipsets recursively
type GraphSyncFetcher struct {
	exchange    GraphExchange
	validator   consensus.BlockSyntaxValidator
	store       bstore.Blockstore
	ssb         selectorbuilder.SelectorSpecBuilder
	peerTracker graphsyncFallbackPeerTracker
}

// NewGraphSyncFetcher returns a GraphsyncFetcher wired up to the input Graphsync exchange and
// attached local blockservice for reloading blocks in memory once they are returned
func NewGraphSyncFetcher(ctx context.Context, exchange GraphExchange, blockstore bstore.Blockstore,
	bv consensus.BlockSyntaxValidator, pt graphsyncFallbackPeerTracker) *GraphSyncFetcher {
	gsf := &GraphSyncFetcher{
		store:       blockstore,
		validator:   bv,
		exchange:    exchange,
		ssb:         selectorbuilder.NewSelectorSpecBuilder(ipldfree.NodeBuilder()),
		peerTracker: pt,
	}
	return gsf
}

// Graphsync can fetch a fixed number of tipsets from a remote peer recursively
// with a single request. We don't know until we get all of the response whether
// our final tipset was included in the response
//
// When fetching tipsets we try to balance performance for two competing cases:
// - an initial chain sync that is likely to fetch lots and lots of tipsets
// - a future update sync that is likely to fetch only a few
//
// To do this, the Graphsync fetcher starts fetching a single tipset at a time,
// then gradually ramps up to fetch lots of tipsets at once, up to a fixed limit
//
// The constants below determine the maximum number of tipsets fetched at once
// (maxRecursionDepth) and how fast the ramp up is (recursionMultipler)
const maxRecursionDepth = 64
const recursionMultiplier = 4

// FetchTipSets gets Tipsets starting from the given tipset key and continuing until
// the done function returns true or errors
//
// For now FetchTipSets operates in two parts:
// 1. It fetches relevant blocks through Graphsync, which writes them to the block store
// 2. It reads them from the block store and validates their syntax as blocks
// and constructs a tipset
// This does have a potentially unwanted side effect of writing blocks to the block store
// that later don't validate (bitswap actually does this as well)
//
// TODO: In the future, the blocks will be validated directly through graphsync as
// go-filecoin migrates to the same IPLD library used by go-graphsync (go-ipld-prime)
//
// See: https://github.com/filecoin-project/go-filecoin/issues/3175
func (gsf *GraphSyncFetcher) FetchTipSets(ctx context.Context, tsKey types.TipSetKey, initialPeer peer.ID, done func(types.TipSet) (bool, error)) ([]types.TipSet, error) {
	rpf := newRequestPeerFinder(gsf.peerTracker, initialPeer)

	// fetch initial tipset
	startingTipset, err := gsf.fetchFirstTipset(ctx, tsKey, rpf)
	if err != nil {
		return nil, err
	}

	// fetch remaining tipsets recursively
	return gsf.fetchRemainingTipsets(ctx, startingTipset, rpf, done)
}

func (gsf *GraphSyncFetcher) fetchFirstTipset(ctx context.Context, tsKey types.TipSetKey, rpf *requestPeerFinder) (types.TipSet, error) {
	cids := tsKey.ToSlice()
	for {
		err := gsf.fetchBlocks(ctx, cids, rpf.CurrentPeer())
		if err == nil {
			break
		}
		// something went wrong in a graphsync request, but we want to keep trying other peers, so
		// just log error
		logGraphsyncFetcher.Infof("Error occurred during Graphsync request: %s", err)
		cids, err = gsf.missingCids(cids)
		if err != nil {
			return types.UndefTipSet, err
		}
		err = rpf.FindNextPeer()
		if err != nil {
			return types.UndefTipSet, fmt.Errorf("Failed fetching tipset: %s", tsKey.String())
		}
	}
	return gsf.loadTipsetFromCids(ctx, tsKey.ToSlice())
}

func (gsf *GraphSyncFetcher) fetchRemainingTipsets(ctx context.Context, startingTipset types.TipSet, rpf *requestPeerFinder, done func(types.TipSet) (bool, error)) ([]types.TipSet, error) {
	out := []types.TipSet{startingTipset}
	isDone, err := done(startingTipset)
	if err != nil {
		return nil, err
	}

	// fetch remaining tipsets recursively
	recursionDepth := 1
	ts := startingTipset
	for !isDone {
		// Because a graphsync query always starts from a single CID,
		// we fetch tipsets starting from the first block in the last tipset and
		// recursively getting sets of parents
		err := gsf.fetchBlocksRecursively(ctx, ts.At(0).Cid(), rpf.CurrentPeer(), recursionDepth)
		if err != nil {
			// something went wrong in a graphsync request, but we want to keep trying other peers, so
			// just log error
			logGraphsyncFetcher.Infof("Error occurred during Graphsync request: %s", err)
		}
		cidsAreMissing := false
		for i := 0; i < recursionDepth; i++ {
			tsKey, err := ts.Parents()
			if err != nil {
				return nil, err
			}
			cidsAreMissing, err = gsf.areCidsMissing(tsKey)
			if err != nil {
				return nil, err
			}
			if cidsAreMissing {
				err := rpf.FindNextPeer()
				if err != nil {
					return nil, fmt.Errorf("Failed fetching tipset: %s", tsKey.String())
				}
				break
			}
			ts, err = gsf.loadTipsetFromCids(ctx, tsKey.ToSlice())
			if err != nil {
				return nil, err
			}
			out = append(out, ts)
			isDone, err = done(ts)
			if err != nil {
				return nil, err
			}
			if isDone {
				break
			}
		}
		if !cidsAreMissing && recursionDepth < maxRecursionDepth {
			recursionDepth *= recursionMultiplier
		}
	}
	return out, nil
}

// fetchBlocks requests a single set of cids as individual blocks, fetching
// non-recursively
func (gsf *GraphSyncFetcher) fetchBlocks(ctx context.Context, cids []cid.Cid, targetPeer peer.ID) error {
	selector := gsf.ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("messages", gsf.ssb.Matcher())
		efsb.Insert("messageReceipts", gsf.ssb.Matcher())
	}).Node()
	errChans := make([]<-chan error, 0, len(cids))
	requestCtx, requestCancel := context.WithCancel(ctx)
	defer requestCancel()
	for _, c := range cids {
		_, errChan := gsf.exchange.Request(requestCtx, targetPeer, cidlink.Link{Cid: c}, selector)
		errChans = append(errChans, errChan)
	}
	for _, errChan := range errChans {
		for err := range errChan {
			return err
		}
	}
	return nil
}

// fetchBlocksRecursively gets the blocks from recursionDepth ancestor tipsets
// starting from baseCid.
func (gsf *GraphSyncFetcher) fetchBlocksRecursively(ctx context.Context, baseCid cid.Cid, targetPeer peer.ID, recursionDepth int) error {
	requestCtx, requestCancel := context.WithCancel(ctx)
	defer requestCancel()

	// recursive selector to fetch n sets of parent blocks
	// starting from block matching base cid:
	//   - fetch all parent blocks
	//   - with exactly the first parent block, repeat again for its parents
	//   - continue up to recursion depth
	selector := gsf.ssb.ExploreRecursive(recursionDepth, gsf.ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("messages", gsf.ssb.Matcher())
		efsb.Insert("messageReceipts", gsf.ssb.Matcher())
		efsb.Insert("parents", gsf.ssb.ExploreUnion(
			gsf.ssb.ExploreAll(gsf.ssb.Matcher()),
			gsf.ssb.ExploreIndex(0, gsf.ssb.ExploreRecursiveEdge()),
		))
	})).Node()

	_, errChan := gsf.exchange.Request(requestCtx, targetPeer, cidlink.Link{Cid: baseCid}, selector)
	for err := range errChan {
		return err
	}
	return nil
}

func (gsf *GraphSyncFetcher) loadTipsetFromCids(ctx context.Context, cids []cid.Cid) (types.TipSet, error) {
	blks := make([]blocks.Block, 0, len(cids))
	for _, cid := range cids {
		block, err := gsf.store.Get(cid)
		if err != nil {
			return types.UndefTipSet, err
		}
		blks = append(blks, block)
	}
	validatedBlocks, err := sanitizeBlocks(ctx, blks, gsf.validator)
	if err != nil {
		return types.UndefTipSet, err
	}

	return types.NewTipSet(validatedBlocks...)
}

func (gsf *GraphSyncFetcher) missingCids(cids []cid.Cid) ([]cid.Cid, error) {
	missing := make([]cid.Cid, 0, len(cids))
	for _, cid := range cids {
		hasBlock, err := gsf.store.Has(cid)
		if err != nil {
			return nil, err
		}
		if !hasBlock {
			missing = append(missing, cid)
		}
	}
	return missing, nil
}

func (gsf *GraphSyncFetcher) areCidsMissing(tsKey types.TipSetKey) (bool, error) {
	missing, err := gsf.missingCids(tsKey.ToSlice())
	if err != nil {
		return false, err
	}
	if len(missing) != 0 {
		return true, nil
	}
	return false, nil
}

type requestPeerFinder struct {
	peerTracker graphsyncFallbackPeerTracker
	currentPeer peer.ID
	triedPeers  map[peer.ID]struct{}
}

func newRequestPeerFinder(peerTracker graphsyncFallbackPeerTracker, initialPeer peer.ID) *requestPeerFinder {
	pri := &requestPeerFinder{peerTracker, initialPeer, make(map[peer.ID]struct{})}
	pri.triedPeers[initialPeer] = struct{}{}
	return pri
}

func (pri *requestPeerFinder) CurrentPeer() peer.ID {
	return pri.currentPeer
}

func (pri *requestPeerFinder) FindNextPeer() error {
	chains := pri.peerTracker.List()
	for _, chain := range chains {
		if _, tried := pri.triedPeers[chain.Peer]; !tried {
			pri.triedPeers[chain.Peer] = struct{}{}
			pri.currentPeer = chain.Peer
			return nil
		}
	}
	return fmt.Errorf("Unable to find any untried peers")
}
