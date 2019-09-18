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
	"github.com/filecoin-project/go-filecoin/types"
)

var logGraphsyncFetcher = logging.Logger("net.graphsync_fetcher")

const (
	// Timeout for a single graphsync request getting "stuck"
	// -- if no more responses are received for a period greater than this,
	// we will assume the request has hung-up and cancel it
	unresponsiveTimeout = 10 * time.Second
)

// Fetcher defines an interface that may be used to fetch data from the network.
type Fetcher interface {
	// FetchTipSets will only fetch TipSets that evaluate to `false` when passed to `done`,
	// this includes the provided `ts`. The TipSet that evaluates to true when
	// passed to `done` will be in the returned slice. The returns slice of TipSets is in Traversal order.
	FetchTipSets(context.Context, types.TipSetKey, peer.ID, func(types.TipSet) (bool, error)) ([]types.TipSet, error)
}

// interface conformance check
var _ Fetcher = (*GraphSyncFetcher)(nil)

// GraphExchange is an interface wrapper to Graphsync so it can be stubbed in
// unit testing
type GraphExchange interface {
	Request(ctx context.Context, p peer.ID, root ipld.Link, selector ipld.Node) (<-chan graphsync.ResponseProgress, <-chan error)
}

type graphsyncFallbackPeerTracker interface {
	List() []*types.ChainInfo
	Self() peer.ID
}

// GraphSyncFetcher is used to fetch data over the network.  It is implemented
// using a Graphsync exchange to fetch tipsets recursively
type GraphSyncFetcher struct {
	exchange            GraphExchange
	validator           consensus.SyntaxValidator
	store               bstore.Blockstore
	ssb                 selectorbuilder.SelectorSpecBuilder
	peerTracker         graphsyncFallbackPeerTracker
	unresponsiveTimeout time.Duration
}

// GraphsyncFetcherOption is function that configures graphsync. It should not
// be created directly but should instead generated through an option function like
// UseFetcherTimeout
type GraphsyncFetcherOption func(*GraphSyncFetcher)

// UseUnresponsiveTimeout sets up the GraphsyncFetcher with a different
// unresponsiveness timeout than the default
func UseUnresponsiveTimeout(timeout time.Duration) GraphsyncFetcherOption {
	return func(gsf *GraphSyncFetcher) {
		gsf.unresponsiveTimeout = timeout
	}
}

// NewGraphSyncFetcher returns a GraphsyncFetcher wired up to the input Graphsync exchange and
// attached local blockservice for reloading blocks in memory once they are returned
func NewGraphSyncFetcher(ctx context.Context, exchange GraphExchange, blockstore bstore.Blockstore,
	bv consensus.SyntaxValidator, pt graphsyncFallbackPeerTracker, options ...GraphsyncFetcherOption) *GraphSyncFetcher {
	gsf := &GraphSyncFetcher{
		store:               blockstore,
		validator:           bv,
		exchange:            exchange,
		ssb:                 selectorbuilder.NewSelectorSpecBuilder(ipldfree.NodeBuilder()),
		peerTracker:         pt,
		unresponsiveTimeout: unresponsiveTimeout,
	}
	for _, option := range options {
		option(gsf)
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
func (gsf *GraphSyncFetcher) FetchTipSets(ctx context.Context, tsKey types.TipSetKey, originatingPeer peer.ID, done func(types.TipSet) (bool, error)) ([]types.TipSet, error) {
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
			// A likely case is the peer doesn't have the tipset. When graphsync provides
			// this status we should quiet this log.
			logGraphsyncFetcher.Infof("request failed: %s", err)
		}

		var verifiedTip types.TipSet
		verifiedTip, blocksToFetch, err = gsf.loadAndVerify(ctx, key)
		if err != nil {
			return types.UndefTipSet, err
		}
		if len(blocksToFetch) == 0 {
			return verifiedTip, nil
		}

		logGraphsyncFetcher.Infof("incomplete fetch for initial tipset %s, trying new peer", key)
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

// fetchBlocks requests a single set of cids as individual blocks, fetching
// non-recursively
func (gsf *GraphSyncFetcher) fetchBlocks(ctx context.Context, cids []cid.Cid, targetPeer peer.ID) error {
	selector := gsf.ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("messages", gsf.ssb.Matcher())
		efsb.Insert("messageReceipts", gsf.ssb.Matcher())
	}).Node()
	errChans := make([]<-chan error, 0, len(cids))
	requestChans := make([]<-chan graphsync.ResponseProgress, 0, len(cids))
	cancelFuncs := make([]func(), 0, len(cids))
	for _, c := range cids {
		requestCtx, requestCancel := context.WithCancel(ctx)
		defer requestCancel()
		requestChan, errChan := gsf.exchange.Request(requestCtx, targetPeer, cidlink.Link{Cid: c}, selector)
		errChans = append(errChans, errChan)
		requestChans = append(requestChans, requestChan)
		cancelFuncs = append(cancelFuncs, requestCancel)
	}
	// Any of the multiple parallel requests might fail. Wait for all of them to complete, then
	// return any error (in this case, the last one to be received).
	var anyError error
	for i, errChan := range errChans {
		requestChan := requestChans[i]
		cancelFunc := cancelFuncs[i]
		err := gsf.consumeResponse(requestChan, errChan, cancelFunc)
		if err != nil {
			anyError = err
		}
	}
	return anyError
}

func (gsf *GraphSyncFetcher) consumeResponse(requestChan <-chan graphsync.ResponseProgress, errChan <-chan error, cancelFunc func()) error {
	timer := time.NewTimer(gsf.unresponsiveTimeout)
	var anyError error
	for errChan != nil || requestChan != nil {
		select {
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
			}
			anyError = err
			timer.Reset(gsf.unresponsiveTimeout)
		case _, ok := <-requestChan:
			if !ok {
				requestChan = nil
			}
			timer.Reset(gsf.unresponsiveTimeout)
		case <-timer.C:
			cancelFunc()
		}
	}
	return anyError
}

// fetchBlocksRecursively gets the blocks from recursionDepth ancestor tipsets
// starting from baseCid.
func (gsf *GraphSyncFetcher) fetchBlocksRecursively(ctx context.Context, baseCid cid.Cid, targetPeer peer.ID, recursionDepth int) error {
	requestCtx, requestCancel := context.WithCancel(ctx)
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

	requestChan, errChan := gsf.exchange.Request(requestCtx, targetPeer, cidlink.Link{Cid: baseCid}, selector)
	return gsf.consumeResponse(requestChan, errChan, requestCancel)
}

// Loads the IPLD blocks for all blocks in a tipset, and checks for the presence of the
// message and receipt list structures in the store.
// Returns the tipset if complete. Otherwise it returns UndefTipSet and the CIDs of
// all blocks missing either their header, messages or receipts.
func (gsf *GraphSyncFetcher) loadAndVerify(ctx context.Context, key types.TipSetKey) (types.TipSet, []cid.Cid, error) {
	// Load the block headers that exist.
	incomplete := make(map[cid.Cid]struct{})
	tip, err := gsf.loadTipHeaders(ctx, key, incomplete)
	if err != nil {
		return types.UndefTipSet, nil, err
	}

	err = gsf.loadAndVerifySubComponents(ctx, tip, incomplete,
		func(blk *types.Block) cid.Cid { return blk.Messages }, func(rawBlock blocks.Block) error {
			messages, err := types.DecodeMessages(rawBlock.RawData())
			if err != nil {
				return errors.Wrapf(err, "fetched data (cid %s) was not a message collection", rawBlock.Cid().String())
			}
			if err := gsf.validator.ValidateMessagesSyntax(ctx, messages); err != nil {
				return errors.Wrapf(err, "invalid messages for for message collection (cid %s)", rawBlock.Cid())
			}
			return nil
		})
	if err != nil {
		return types.UndefTipSet, nil, err
	}

	err = gsf.loadAndVerifySubComponents(ctx, tip, incomplete,
		func(blk *types.Block) cid.Cid { return blk.MessageReceipts }, func(rawBlock blocks.Block) error {
			receipts, err := types.DecodeReceipts(rawBlock.RawData())
			if err != nil {
				return errors.Wrapf(err, "fetched data (cid %s) was not a message receipt collection", rawBlock.Cid().String())
			}
			if err := gsf.validator.ValidateReceiptsSyntax(ctx, receipts); err != nil {
				return errors.Wrapf(err, "invalid receipts for for receipt collection (cid %s)", rawBlock.Cid())
			}
			return nil
		})

	if err != nil {
		return types.UndefTipSet, nil, err
	}

	if len(incomplete) > 0 {
		incompleteArr := make([]cid.Cid, 0, len(incomplete))
		for cid := range incomplete {
			incompleteArr = append(incompleteArr, cid)
		}
		return types.UndefTipSet, incompleteArr, nil
	}

	return tip, nil, nil
}

// Loads and validates the block headers for a tipset. Returns the tipset if complete,
// else the cids of blocks which are not yet stored.
func (gsf *GraphSyncFetcher) loadTipHeaders(ctx context.Context, key types.TipSetKey, incomplete map[cid.Cid]struct{}) (types.TipSet, error) {
	rawBlocks := make([]blocks.Block, 0, key.Len())
	for it := key.Iter(); !it.Complete(); it.Next() {
		hasBlock, err := gsf.store.Has(it.Value())
		if err != nil {
			return types.UndefTipSet, err
		}
		if !hasBlock {
			incomplete[it.Value()] = struct{}{}
			continue
		}
		rawBlock, err := gsf.store.Get(it.Value())
		if err != nil {
			return types.UndefTipSet, err
		}
		rawBlocks = append(rawBlocks, rawBlock)
	}

	// Validate the headers.
	validatedBlocks, err := sanitizeBlocks(ctx, rawBlocks, gsf.validator)
	if err != nil || len(validatedBlocks) == 0 {
		return types.UndefTipSet, err
	}
	tip, err := types.NewTipSet(validatedBlocks...)
	return tip, err
}

type getBlockComponentFn func(*types.Block) cid.Cid
type verifyComponentFn func(blocks.Block) error

// Loads and validates the block messages for a tipset. Returns the tipset if complete,
// else the cids of blocks which are not yet stored.
func (gsf *GraphSyncFetcher) loadAndVerifySubComponents(ctx context.Context,
	tip types.TipSet,
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
		if _, tried := pri.triedPeers[chain.Peer]; !tried {
			pri.triedPeers[chain.Peer] = struct{}{}
			pri.currentPeer = chain.Peer
			return nil
		}
	}
	return fmt.Errorf("Unable to find any untried peers")
}

func sanitizeBlocks(ctx context.Context, unsanitized []blocks.Block, validator consensus.BlockSyntaxValidator) ([]*types.Block, error) {
	var blocks []*types.Block
	for _, u := range unsanitized {
		block, err := types.DecodeBlock(u.RawData())
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
