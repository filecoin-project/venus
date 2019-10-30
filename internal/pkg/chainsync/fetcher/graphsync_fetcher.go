package fetcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/go-amt-ipld"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/internal/syncer"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/ipld/go-ipld-prime"
	ipldfree "github.com/ipld/go-ipld-prime/impl/free"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	selectorbuilder "github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	typegen "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

var logGraphsyncFetcher = logging.Logger("chainsync.fetcher.graphsync")

const (
	// Timeout for a single graphsync request getting "stuck"
	// -- if no more responses are received for a period greater than this,
	// we will assume the request has hung-up and cancel it
	progressTimeout = 10 * time.Second

	// AMT selector recursion. An AMT has arity of 8 so this gives allows
	// us to retrieve trees with 8^10 (1,073,741,824) elements.
	amtRecurstionDepth = uint32(10)

	// field index of AMT node in AMT head
	amtHeadNodeFieldIndex = 2

	// field index of links array AMT node
	amtNodeLinksFieldIndex = 1

	// field index of values array AMT node
	amtNodeValuesFieldIndex = 2
)

// interface conformance check
var _ syncer.Fetcher = (*GraphSyncFetcher)(nil)

// GraphExchange is an interface wrapper to Graphsync so it can be stubbed in
// unit testing
type GraphExchange interface {
	Request(ctx context.Context, p peer.ID, root ipld.Link, selector ipld.Node) (<-chan graphsync.ResponseProgress, <-chan error)
}

type graphsyncFallbackPeerTracker interface {
	List() []*block.ChainInfo
	Self() peer.ID
}

// GraphSyncFetcher is used to fetch data over the network.  It is implemented
// using a Graphsync exchange to fetch tipsets recursively
type GraphSyncFetcher struct {
	exchange    GraphExchange
	validator   consensus.SyntaxValidator
	store       bstore.Blockstore
	ssb         selectorbuilder.SelectorSpecBuilder
	peerTracker graphsyncFallbackPeerTracker
	systemClock clock.Clock
}

// NewGraphSyncFetcher returns a GraphsyncFetcher wired up to the input Graphsync exchange and
// attached local blockservice for reloading blocks in memory once they are returned
func NewGraphSyncFetcher(ctx context.Context, exchange GraphExchange, blockstore bstore.Blockstore,
	bv consensus.SyntaxValidator, systemClock clock.Clock, pt graphsyncFallbackPeerTracker) *GraphSyncFetcher {
	gsf := &GraphSyncFetcher{
		store:       blockstore,
		validator:   bv,
		exchange:    exchange,
		ssb:         selectorbuilder.NewSelectorSpecBuilder(ipldfree.NodeBuilder()),
		peerTracker: pt,
		systemClock: systemClock,
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
func (gsf *GraphSyncFetcher) FetchTipSets(ctx context.Context, tsKey block.TipSetKey, originatingPeer peer.ID, done func(block.TipSet) (bool, error)) ([]block.TipSet, error) {
	return gsf.fetchTipSetsCommon(ctx, tsKey, originatingPeer, done, gsf.loadAndVerifyFullBlock, gsf.fullBlockSel, gsf.recFullBlockSel)
}

// FetchTipSetHeaders behaves as FetchTipSets but it only fetches and
// syntactically validates a chain of headers, not full blocks.
func (gsf *GraphSyncFetcher) FetchTipSetHeaders(ctx context.Context, tsKey block.TipSetKey, originatingPeer peer.ID, done func(block.TipSet) (bool, error)) ([]block.TipSet, error) {
	return gsf.fetchTipSetsCommon(ctx, tsKey, originatingPeer, done, gsf.loadAndVerifyHeader, gsf.headerSel, gsf.recHeaderSel)
}

func (gsf *GraphSyncFetcher) fetchTipSetsCommon(ctx context.Context, tsKey block.TipSetKey, originatingPeer peer.ID, done func(block.TipSet) (bool, error), loadAndVerify func(context.Context, block.TipSetKey) (block.TipSet, []cid.Cid, error), selGen func() ipld.Node, recSelGen func(int) ipld.Node) ([]block.TipSet, error) {
	// We can run into issues if we fetch from an originatingPeer that we
	// are not already connected to so we usually ignore this value.
	// However if the originator is our own peer ID (i.e. this node mined
	// the block) then we need to fetch from ourselves to retrieve it
	fetchFromSelf := originatingPeer == gsf.peerTracker.Self()
	rpf, err := newRequestPeerFinder(gsf.peerTracker, fetchFromSelf)
	if err != nil {
		return nil, err
	}

	// fetch initial tipset
	startingTipset, err := gsf.fetchFirstTipset(ctx, tsKey, loadAndVerify, selGen, rpf)
	if err != nil {
		return nil, err
	}

	// fetch remaining tipsets recursively
	return gsf.fetchRemainingTipsets(ctx, startingTipset, done, loadAndVerify, recSelGen, rpf)
}

func (gsf *GraphSyncFetcher) fetchFirstTipset(ctx context.Context, tsKey block.TipSetKey, loadAndVerify func(context.Context, block.TipSetKey) (block.TipSet, []cid.Cid, error), selGen func() ipld.Node, rpf *requestPeerFinder) (block.TipSet, error) {
	blocksToFetch := tsKey.ToSlice()
	for {
		peer := rpf.CurrentPeer()
		logGraphsyncFetcher.Infof("fetching initial tipset %s from peer %s", tsKey, peer)
		err := gsf.fetchBlocks(ctx, selGen, blocksToFetch, peer)
		if err != nil {
			// A likely case is the peer doesn't have the tipset. When graphsync provides
			// this status we should quiet this log.
			logGraphsyncFetcher.Infof("request failed: %s", err)
		}

		var verifiedTip block.TipSet
		verifiedTip, blocksToFetch, err = loadAndVerify(ctx, tsKey)
		if err != nil {
			return block.UndefTipSet, err
		}
		if len(blocksToFetch) == 0 {
			return verifiedTip, nil
		}

		logGraphsyncFetcher.Infof("incomplete fetch for initial tipset %s, trying new peer", tsKey)
		// Some of the blocks may have been fetched, but avoid tricksy optimization here and just
		// request the whole bunch again. Graphsync internally will avoid redundant network requests.
		err = rpf.FindNextPeer()
		if err != nil {
			return block.UndefTipSet, errors.Wrapf(err, "fetching tipset: %s", tsKey)
		}
	}
}

func (gsf *GraphSyncFetcher) fetchRemainingTipsets(ctx context.Context, startingTipset block.TipSet, done func(block.TipSet) (bool, error), loadAndVerify func(context.Context, block.TipSetKey) (block.TipSet, []cid.Cid, error), recSelGen func(int) ipld.Node, rpf *requestPeerFinder) ([]block.TipSet, error) {
	out := []block.TipSet{startingTipset}
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
		err := gsf.fetchBlocksRecursively(ctx, recSelGen, childBlock.Cid(), peer, recursionDepth)
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

			var verifiedTip block.TipSet
			verifiedTip, incomplete, err = loadAndVerify(ctx, tsKey)
			if err != nil {
				return nil, err
			}
			if len(incomplete) == 0 {
				out = append(out, verifiedTip)
				isDone, err = done(verifiedTip)
				if err != nil {
					return nil, err
				}
				anchor = verifiedTip
			} else {
				logGraphsyncFetcher.Infof("incomplete fetch for tipset %s, trying new peer", tsKey)
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

// fullBlockSel is a function that generates a selector for a block and its messages.
func (gsf *GraphSyncFetcher) fullBlockSel() ipld.Node {
	selector := gsf.ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("messages", gsf.ssb.ExploreFields(func(messagesSelector selectorbuilder.ExploreFieldsSpecBuilder) {
			messagesSelector.Insert("secpRoot", gsf.fetchThroughAMTSelector(amtRecurstionDepth))
			messagesSelector.Insert("bLSRoot", gsf.fetchThroughAMTSelector(amtRecurstionDepth))
		}))
	}).Node()
	return selector
}

// headerSel is a function that generates a selector for a block header.
func (gsf *GraphSyncFetcher) headerSel() ipld.Node {
	return gsf.ssb.Matcher().Node()
}

// fetchBlocks requests a single set of cids as individual blocks, fetching
// non-recursively
func (gsf *GraphSyncFetcher) fetchBlocks(ctx context.Context, selGen func() ipld.Node, cids []cid.Cid, targetPeer peer.ID) error {
	selector := selGen()
	var wg sync.WaitGroup
	// Any of the multiple parallel requests might fail. Wait for all of them to complete, then
	// return any error (in this case, the first one to be received).
	var setAnyError sync.Once
	var anyError error
	for _, c := range cids {
		requestCtx, requestCancel := context.WithCancel(ctx)
		defer requestCancel()
		requestChan, errChan := gsf.exchange.Request(requestCtx, targetPeer, cidlink.Link{Cid: c}, selector)
		wg.Add(1)
		go func(requestChan <-chan graphsync.ResponseProgress, errChan <-chan error, cancelFunc func()) {
			defer wg.Done()
			err := gsf.consumeResponse(requestChan, errChan, cancelFunc)
			if err != nil {
				setAnyError.Do(func() {
					anyError = err
				})
			}
		}(requestChan, errChan, requestCancel)
	}
	wg.Wait()
	return anyError
}

func (gsf *GraphSyncFetcher) fetchThroughAMTSelector(recursionDepth uint32) selectorbuilder.SelectorSpec {
	return gsf.ssb.ExploreIndex(amtHeadNodeFieldIndex,
		gsf.ssb.ExploreRecursive(int(recursionDepth),
			gsf.ssb.ExploreUnion(
				gsf.ssb.ExploreIndex(amtNodeLinksFieldIndex, gsf.ssb.ExploreAll(gsf.ssb.ExploreRecursiveEdge())),
				gsf.ssb.ExploreIndex(amtNodeValuesFieldIndex, gsf.ssb.ExploreAll(gsf.ssb.Matcher())))))
}

func (gsf *GraphSyncFetcher) consumeResponse(requestChan <-chan graphsync.ResponseProgress, errChan <-chan error, cancelFunc func()) error {
	timer := gsf.systemClock.NewTimer(progressTimeout)
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

// recFullBlockSel  generates a selector for a chain of full blocks including
// messages.
func (gsf *GraphSyncFetcher) recFullBlockSel(recursionDepth int) ipld.Node {
	// recursive selector to fetch n sets of parent blocks
	// starting from block matching base cid:
	//   - fetch all parent blocks, with messages
	//   - with exactly the first parent block, repeat again for its parents
	//   - continue up to recursion depth
	selector := gsf.ssb.ExploreRecursive(recursionDepth, gsf.ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("parents", gsf.ssb.ExploreUnion(
			gsf.ssb.ExploreAll(
				gsf.ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
					efsb.Insert("messages", gsf.ssb.ExploreFields(func(messagesSelector selectorbuilder.ExploreFieldsSpecBuilder) {
						messagesSelector.Insert("secpRoot", gsf.fetchThroughAMTSelector(amtRecurstionDepth))
						messagesSelector.Insert("bLSRoot", gsf.fetchThroughAMTSelector(amtRecurstionDepth))
					}))
				}),
			),
			gsf.ssb.ExploreIndex(0, gsf.ssb.ExploreRecursiveEdge()),
		))
	})).Node()
	return selector
}

// recHeaderSel generates a selector for a chain of only block headers.
func (gsf *GraphSyncFetcher) recHeaderSel(recursionDepth int) ipld.Node {
	selector := gsf.ssb.ExploreRecursive(recursionDepth, gsf.ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("parents", gsf.ssb.ExploreUnion(
			gsf.ssb.ExploreAll(
				gsf.ssb.Matcher(),
			),
			gsf.ssb.ExploreIndex(0, gsf.ssb.ExploreRecursiveEdge()),
		))
	})).Node()
	return selector
}

// fetchBlocksRecursively gets the blocks from recursionDepth ancestor tipsets
// starting from baseCid.
func (gsf *GraphSyncFetcher) fetchBlocksRecursively(ctx context.Context, recSelGen func(int) ipld.Node, baseCid cid.Cid, targetPeer peer.ID, recursionDepth int) error {
	requestCtx, requestCancel := context.WithCancel(ctx)
	defer requestCancel()
	selector := recSelGen(recursionDepth)

	requestChan, errChan := gsf.exchange.Request(requestCtx, targetPeer, cidlink.Link{Cid: baseCid}, selector)
	return gsf.consumeResponse(requestChan, errChan, requestCancel)
}

// loadAndVerifyHeaders loads the IPLD blocks for the headers in a tipset.
// It returns the tipset if complete. Otherwise it returns UndefTipset and the
// CIDs of all missing headers.
func (gsf *GraphSyncFetcher) loadAndVerifyHeader(ctx context.Context, key block.TipSetKey) (block.TipSet, []cid.Cid, error) {
	// Load the block headers that exist.
	incomplete := make(map[cid.Cid]struct{})
	tip, err := gsf.loadTipHeaders(ctx, key, incomplete)
	if err != nil {
		return block.UndefTipSet, nil, err
	}
	if len(incomplete) == 0 {
		return tip, nil, nil
	}
	incompleteArr := make([]cid.Cid, 0, len(incomplete))
	for cid := range incomplete {
		incompleteArr = append(incompleteArr, cid)
	}
	return block.UndefTipSet, incompleteArr, nil
}

// Loads the IPLD blocks for all blocks in a tipset, and checks for the presence of the
// message list structures in the store.
// Returns the tipset if complete. Otherwise it returns UndefTipSet and the CIDs of
// all blocks missing either their header or messages.
func (gsf *GraphSyncFetcher) loadAndVerifyFullBlock(ctx context.Context, key block.TipSetKey) (block.TipSet, []cid.Cid, error) {
	// Load the block headers that exist.
	incomplete := make(map[cid.Cid]struct{})
	tip, err := gsf.loadTipHeaders(ctx, key, incomplete)
	if err != nil {
		return block.UndefTipSet, nil, err
	}

	err = gsf.loadAndVerifySubComponents(ctx, tip, incomplete,
		func(blk *block.Block) cid.Cid { return blk.Messages.SecpRoot }, func(rawBlock blocks.Block) error {
			messages := []*types.SignedMessage{}

			err := gsf.loadAndProcessAMTData(ctx, rawBlock.Cid(), func(msgBlock blocks.Block) error {
				var message types.SignedMessage
				if err := cbor.DecodeInto(msgBlock.RawData(), &message); err != nil {
					return errors.Wrapf(err, "could not decode secp message (cid %s)", msgBlock.Cid())
				}
				messages = append(messages, &message)
				return nil
			})
			if err != nil {
				return err
			}

			if err := gsf.validator.ValidateMessagesSyntax(ctx, messages); err != nil {
				return errors.Wrapf(err, "invalid messages for for message collection (cid %s)", rawBlock.Cid())
			}
			return nil
		})
	if err != nil {
		return block.UndefTipSet, nil, err
	}

	err = gsf.loadAndVerifySubComponents(ctx, tip, incomplete,
		func(blk *block.Block) cid.Cid { return blk.Messages.BLSRoot }, func(rawBlock blocks.Block) error {
			messages := []*types.UnsignedMessage{}

			err := gsf.loadAndProcessAMTData(ctx, rawBlock.Cid(), func(msgBlock blocks.Block) error {
				var message types.UnsignedMessage
				if err := cbor.DecodeInto(msgBlock.RawData(), &message); err != nil {
					return errors.Wrapf(err, "could not decode bls message (cid %s)", msgBlock.Cid())
				}
				messages = append(messages, &message)
				return nil
			})
			if err != nil {
				return err
			}

			if err := gsf.validator.ValidateUnsignedMessagesSyntax(ctx, messages); err != nil {
				return errors.Wrapf(err, "invalid messages for for message collection (cid %s)", rawBlock.Cid())
			}
			return nil
		})
	if err != nil {
		return block.UndefTipSet, nil, err
	}

	if len(incomplete) > 0 {
		incompleteArr := make([]cid.Cid, 0, len(incomplete))
		for cid := range incomplete {
			incompleteArr = append(incompleteArr, cid)
		}
		return block.UndefTipSet, incompleteArr, nil
	}

	return tip, nil, nil
}

// loadAndProcessAMTData processes data loaded from an AMT that is stored in the fetcher's datastore.
func (gsf *GraphSyncFetcher) loadAndProcessAMTData(ctx context.Context, c cid.Cid, processFn func(b blocks.Block) error) error {
	as := amt.WrapBlockstore(gsf.store)

	a, err := amt.LoadAMT(as, c)
	if err != nil {
		if err == bstore.ErrNotFound {
			return err
		}
		return errors.Wrapf(err, "fetched data (cid %s) could not be decoded as an AMT", c.String())
	}

	return a.ForEach(func(index uint64, deferred *typegen.Deferred) error {
		var c cid.Cid
		if err := cbor.DecodeInto(deferred.Raw, &c); err != nil {
			return errors.Wrapf(err, "cid from amt could not be decoded as a Cid (index %d)", index)
		}

		ok, err := gsf.store.Has(c)
		if err != nil {
			return errors.Wrapf(err, "could not retrieve secp message from blockstore (cid %s)", c)
		}

		if !ok {
			return bstore.ErrNotFound
		}

		rawMsg, err := gsf.store.Get(c)
		if err != nil {
			return errors.Wrapf(err, "could not retrieve secp message from blockstore (cid %s)", c)
		}

		if err := processFn(rawMsg); err != nil {
			return errors.Wrapf(err, "could not decode secp message (cid %s)", c)
		}

		return nil
	})
}

// Loads and validates the block headers for a tipset. Returns the tipset if complete,
// else the cids of blocks which are not yet stored.
func (gsf *GraphSyncFetcher) loadTipHeaders(ctx context.Context, key block.TipSetKey, incomplete map[cid.Cid]struct{}) (block.TipSet, error) {
	rawBlocks := make([]blocks.Block, 0, key.Len())
	for it := key.Iter(); !it.Complete(); it.Next() {
		hasBlock, err := gsf.store.Has(it.Value())
		if err != nil {
			return block.UndefTipSet, err
		}
		if !hasBlock {
			incomplete[it.Value()] = struct{}{}
			continue
		}
		rawBlock, err := gsf.store.Get(it.Value())
		if err != nil {
			return block.UndefTipSet, err
		}
		rawBlocks = append(rawBlocks, rawBlock)
	}

	// Validate the headers.
	validatedBlocks, err := sanitizeBlocks(ctx, rawBlocks, gsf.validator)
	if err != nil || len(validatedBlocks) == 0 {
		return block.UndefTipSet, err
	}
	tip, err := block.NewTipSet(validatedBlocks...)
	return tip, err
}

type getBlockComponentFn func(*block.Block) cid.Cid
type verifyComponentFn func(blocks.Block) error

// Loads and validates the block messages for a tipset. Returns the tipset if complete,
// else the cids of blocks which are not yet stored.
func (gsf *GraphSyncFetcher) loadAndVerifySubComponents(ctx context.Context,
	tip block.TipSet,
	incomplete map[cid.Cid]struct{},
	getBlockComponent getBlockComponentFn,
	verifyComponent verifyComponentFn) error {
	subComponents := make([]blocks.Block, 0, tip.Len())

	// Check that nested structures are also stored, recording any that are missing as incomplete.
	for i := 0; i < tip.Len(); i++ {
		blk := tip.At(i)
		link := getBlockComponent(blk)
		ok, err := gsf.store.Has(link)
		if err != nil {
			return err
		}
		if !ok {
			incomplete[blk.Cid()] = struct{}{}
			continue
		}
		rawBlock, err := gsf.store.Get(link)
		if err != nil {
			return err
		}
		subComponents = append(subComponents, rawBlock)
	}

	for _, rawBlock := range subComponents {
		err := verifyComponent(rawBlock)
		if err != nil {
			// If this is a not found error, this simply means we failed to fetch some information.
			// Mark this block as incomplete, but don't fail.
			if err == bstore.ErrNotFound {
				incomplete[rawBlock.Cid()] = struct{}{}
				return nil
			}
			return err
		}
	}

	return nil
}

type requestPeerFinder struct {
	peerTracker graphsyncFallbackPeerTracker
	currentPeer peer.ID
	triedPeers  map[peer.ID]struct{}
}

func newRequestPeerFinder(peerTracker graphsyncFallbackPeerTracker, fetchFromSelf bool) (*requestPeerFinder, error) {
	pri := &requestPeerFinder{
		peerTracker: peerTracker,
		triedPeers:  make(map[peer.ID]struct{}),
	}

	// If the new cid triggering this request came from ourselves then
	// the first peer to request from should be ourselves.
	if fetchFromSelf {
		pri.triedPeers[peerTracker.Self()] = struct{}{}
		pri.currentPeer = peerTracker.Self()
		return pri, nil
	}

	// Get a peer ID from the peer tracker
	err := pri.FindNextPeer()
	if err != nil {
		return nil, err
	}
	return pri, nil
}

func (pri *requestPeerFinder) CurrentPeer() peer.ID {
	return pri.currentPeer
}

func (pri *requestPeerFinder) FindNextPeer() error {
	chains := pri.peerTracker.List()
	for _, chain := range chains {
		if _, tried := pri.triedPeers[chain.Sender]; !tried {
			pri.triedPeers[chain.Sender] = struct{}{}
			pri.currentPeer = chain.Sender
			return nil
		}
	}
	return fmt.Errorf("Unable to find any untried peers")
}

func sanitizeBlocks(ctx context.Context, unsanitized []blocks.Block, validator consensus.BlockSyntaxValidator) ([]*block.Block, error) {
	var blocks []*block.Block
	for _, u := range unsanitized {
		block, err := block.DecodeBlock(u.RawData())
		if err != nil {
			return nil, errors.Wrapf(err, "fetched data (cid %s) was not a block", u.Cid().String())
		}

		if err := validator.ValidateSyntax(ctx, block); err != nil {
			return nil, errors.Wrapf(err, "invalid block %s", block.Cid())
		}

		blocks = append(blocks, block)
	}
	return blocks, nil
}
