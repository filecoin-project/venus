package net

import (
	"context"
	"fmt"
	"time"

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
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/types"
)

var (
	tsSize       = metrics.NewInt64ByteBucket("net/tipset_size", "The size in bytes of a tipset")
	fetchTsTimer = metrics.NewTimerMs("net/fetch_tipset", "Duration of tipset fetching in milliseconds")
)

var logGraphsyncFetcher = logging.Logger("net.graphsync_fetcher")

const (
	// Timeout for a single graphsync request (which may be for many blocks).
	// We might prefer this timeout to scale with the number of blocks expected in the fetch,
	// when that number is large.
	requestTimeout = 60 * time.Second
)

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
	sw := fetchTsTimer.Start(ctx)
	defer sw.Stop(ctx)

	rpf := newRequestPeerFinder(gsf.peerTracker, initialPeer)

	// fetch initial tipset
	startingTipset, err := gsf.fetchFirstTipset(ctx, tsKey, rpf)
	if err != nil {
		return nil, err
	}

	// fetch remaining tipsets recursively
	return gsf.fetchRemainingTipsets(ctx, startingTipset, rpf, done)
}

func (gsf *GraphSyncFetcher) fetchFirstTipset(ctx context.Context, key types.TipSetKey, rpf *requestPeerFinder) (types.TipSet, error) {
	blocksToFetch := key.ToSlice()
	for {
		peer := rpf.CurrentPeer()
		logGraphsyncFetcher.Infof("fetching initial tipset %s from peer %s", key, peer)
		err := gsf.fetchBlocks(ctx, blocksToFetch, peer)
		if err != nil {
			logGraphsyncFetcher.Infof("request failed: %s", err)
		}

		var verifiedTip types.TipSet
		verifiedTip, blocksToFetch, err = gsf.loadAndVerify(ctx, key)
		if err != nil {
			return types.UndefTipSet, err
		}
		tsSize.Add(ctx, int64(verifiedTip.Size()))
		if len(blocksToFetch) == 0 {
			return verifiedTip, nil
		}

		logGraphsyncFetcher.Warningf("incomplete fetch for first tipset %s, trying new peer", key)
		// Some of the blocks may have been fetched, but avoid tricksy optimization here and just
		// request the whole bunch again. Graphsync internally will avoid redundant network requests.
		err = rpf.FindNextPeer()
		if err != nil {
			return types.UndefTipSet, errors.Wrapf(err, "fetching tipset: %s", key)
		}
	}
}

func (gsf *GraphSyncFetcher) fetchRemainingTipsets(ctx context.Context, startingTipset types.TipSet, rpf *requestPeerFinder, done func(types.TipSet) (bool, error)) ([]types.TipSet, error) {
	out := []types.TipSet{startingTipset}
	isDone, err := done(startingTipset)
	if err != nil {
		return nil, err
	}

	// fetch remaining tipsets recursively
	recursionDepth := 1
	anchor := startingTipset // The tipset above the one we actually want to fetch.
	for !isDone {
		// Because a graphsync query always starts from a single CID,
		// we fetch tipsets anchored from any block in the last (i.e. highest) tipset and
		// recursively fetching sets of parents.
		childBlock := anchor.At(0)
		peer := rpf.CurrentPeer()
		logGraphsyncFetcher.Infof("fetching chain from height %d, block %s, peer %s, %d levels", childBlock.Height, childBlock.Cid(), peer, recursionDepth)
		err := gsf.fetchBlocksRecursively(ctx, childBlock.Cid(), peer, recursionDepth)
		if err != nil {
			// something went wrong in a graphsync request, but we want to keep trying other peers, so
			// just log error
			logGraphsyncFetcher.Infof("request failed, trying another peer: %s", err)
		}
		var incomplete []cid.Cid
		for i := 0; !isDone && i < recursionDepth; i++ {
			tsKey, err := anchor.Parents()
			if err != nil {
				return nil, err
			}

			var verifiedTip types.TipSet
			verifiedTip, incomplete, err = gsf.loadAndVerify(ctx, tsKey)
			if err != nil {
				return nil, err
			}
			tsSize.Add(ctx, int64(verifiedTip.Size()))
			if len(incomplete) == 0 {
				out = append(out, verifiedTip)
				isDone, err = done(verifiedTip)
				if err != nil {
					return nil, err
				}
				anchor = verifiedTip
			} else {
				logGraphsyncFetcher.Warningf("incomplete fetch for tipset %s, trying new peer", tsKey)
				err := rpf.FindNextPeer()
				if err != nil {
					return nil, errors.Wrapf(err, "fetching tipset: %s", tsKey)
				}
				break // Stop verifying, make another fetch
			}
		}
		if len(incomplete) == 0 && recursionDepth < maxRecursionDepth {
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
	requestCtx, requestCancel := context.WithTimeout(ctx, requestTimeout)
	defer requestCancel()
	for _, c := range cids {
		_, errChan := gsf.exchange.Request(requestCtx, targetPeer, cidlink.Link{Cid: c}, selector)
		errChans = append(errChans, errChan)
	}
	// Any of the multiple parallel requests might fail. Wait for all of them to complete, then
	// return any error (in this case, the last one to be received).
	var anyError error
	for _, errChan := range errChans {
		for err := range errChan {
			anyError = err
		}
	}
	return anyError
}

// fetchBlocksRecursively gets the blocks from recursionDepth ancestor tipsets
// starting from baseCid.
func (gsf *GraphSyncFetcher) fetchBlocksRecursively(ctx context.Context, baseCid cid.Cid, targetPeer peer.ID, recursionDepth int) error {
	requestCtx, requestCancel := context.WithTimeout(ctx, requestTimeout)
	defer requestCancel()

	// recursive selector to fetch n sets of parent blocks
	// starting from block matching base cid:
	//   - fetch all parent blocks, with messages/receipts
	//   - with exactly the first parent block, repeat again for its parents
	//   - continue up to recursion depth
	selector := gsf.ssb.ExploreRecursive(recursionDepth, gsf.ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("parents", gsf.ssb.ExploreUnion(
			gsf.ssb.ExploreAll(
				gsf.ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
					efsb.Insert("messages", gsf.ssb.Matcher())
					efsb.Insert("messageReceipts", gsf.ssb.Matcher())
				}),
			),
			gsf.ssb.ExploreIndex(0, gsf.ssb.ExploreRecursiveEdge()),
		))
	})).Node()

	_, errChan := gsf.exchange.Request(requestCtx, targetPeer, cidlink.Link{Cid: baseCid}, selector)
	for err := range errChan {
		return err
	}
	return nil
}

// Loads the IPLD blocks for all blocks in a tipset, and checks for the presence of the
// message and receipt list structures in the store.
// Returns the tipset if complete. Otherwise it returns UndefTipSet and the CIDs of
// all blocks missing either their header, messages or receipts.
func (gsf *GraphSyncFetcher) loadAndVerify(ctx context.Context, key types.TipSetKey) (types.TipSet, []cid.Cid, error) {
	// Load the block headers that exist.
	tip, incomplete, err := gsf.loadTipHeaders(ctx, key)
	if err != nil {
		return types.UndefTipSet, nil, err
	}

	// Check that nested structures are also stored, recording any that are missing as incomplete.
	for i := 0; i < tip.Len(); i++ {
		blk := tip.At(i)
		for _, link := range []cid.Cid{blk.Messages, blk.MessageReceipts} {
			// TODO: validate the structures, don't just check for their presence #3232
			ok, err := gsf.store.Has(link)
			if err != nil {
				return types.UndefTipSet, nil, err
			}
			if !ok {
				incomplete = append(incomplete, blk.Cid())
			}
		}
	}
	if len(incomplete) > 0 {
		tip = types.UndefTipSet
	}
	return tip, incomplete, nil
}

// Loads and validates the block headers for a tipset. Returns the tipset if complete,
// else the cids of blocks which are not yet stored.
func (gsf *GraphSyncFetcher) loadTipHeaders(ctx context.Context, key types.TipSetKey) (types.TipSet, []cid.Cid, error) {
	rawBlocks := make([]blocks.Block, 0, key.Len())
	var incomplete []cid.Cid
	for it := key.Iter(); !it.Complete(); it.Next() {
		hasBlock, err := gsf.store.Has(it.Value())
		if err != nil {
			return types.UndefTipSet, nil, err
		}
		if !hasBlock {
			incomplete = append(incomplete, it.Value())
			continue
		}
		rawBlock, err := gsf.store.Get(it.Value())
		if err != nil {
			return types.UndefTipSet, nil, err
		}
		rawBlocks = append(rawBlocks, rawBlock)
	}

	// Validate the headers.
	validatedBlocks, err := sanitizeBlocks(ctx, rawBlocks, gsf.validator)
	if err != nil || len(validatedBlocks) == 0 {
		return types.UndefTipSet, incomplete, err
	}
	tip, err := types.NewTipSet(validatedBlocks...)
	return tip, incomplete, err
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
