package net

import (
	"context"
	"fmt"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	ipldfree "github.com/ipld/go-ipld-prime/impl/free"
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
	progressTimeout = 10 * time.Second
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

// GraphSyncFetcher is used to fetch data over the network.  It is implemented
// using a Graphsync exchange to fetch tipsets recursively
type GraphSyncFetcher struct {
	exchange  GraphsyncSessionExchange
	validator consensus.SyntaxValidator
	store     bstore.Blockstore
	ssb       selectorbuilder.SelectorSpecBuilder
}

// NewGraphSyncFetcher returns a GraphsyncFetcher wired up to the input Graphsync exchange and
// attached local blockservice for reloading blocks in memory once they are returned
func NewGraphSyncFetcher(ctx context.Context, exchange GraphsyncSessionExchange, blockstore bstore.Blockstore,
	bv consensus.SyntaxValidator) *GraphSyncFetcher {
	gsf := &GraphSyncFetcher{
		exchange:  exchange,
		store:     blockstore,
		validator: bv,
		ssb:       selectorbuilder.NewSelectorSpecBuilder(ipldfree.NodeBuilder()),
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

	gss, err := gsf.exchange.NewSession(ctx, originatingPeer)
	if err != nil {
		return nil, err
	}

	// fetch initial tipset
	startingTipset, err := gsf.fetchFirstTipset(ctx, tsKey, gss)
	if err != nil {
		return nil, err
	}

	// fetch remaining tipsets recursively
	return gsf.fetchRemainingTipsets(ctx, startingTipset, gss, done)
}

func (gsf *GraphSyncFetcher) fetchFirstTipset(ctx context.Context, key types.TipSetKey, gss GraphsyncSession) (types.TipSet, error) {
	blocksToFetch := key.ToSlice()
	selector := gsf.ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("messages", gsf.ssb.Matcher())
		efsb.Insert("messageReceipts", gsf.ssb.Matcher())
	}).Node()

	logGraphsyncFetcher.Infof("fetching initial tipset %s", key)
	err := gss.Request(ctx, blocksToFetch, selector, true)
	if err != nil {
		return types.UndefTipSet, err
	}
	var verifiedTip types.TipSet
	verifiedTip, blocksToFetch, err = gsf.loadAndVerify(ctx, key)
	if err != nil {
		return types.UndefTipSet, err
	}
	if len(blocksToFetch) > 0 {
		// Graphsync session returns w/o error but blocks are still missing?
		// this honestly should never happen given the gaurantees of graphsync
		return types.UndefTipSet, errors.Wrapf(fmt.Errorf("Query returned wrong blocks"), "fetching tipset: %s", key)
	}
	return verifiedTip, nil
}

func (gsf *GraphSyncFetcher) fetchRemainingTipsets(ctx context.Context, startingTipset types.TipSet, gss GraphsyncSession, done func(types.TipSet) (bool, error)) ([]types.TipSet, error) {
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
		logGraphsyncFetcher.Infof("fetching chain from height %d, block %s, %d levels", childBlock.Height, childBlock.Cid(), recursionDepth)
		err := gsf.fetchBlocksRecursively(ctx, childBlock.Cid(), recursionDepth, gss)
		if err != nil {
			return nil, err
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
				break // Stop verifying, make another fetch
			}
		}
		if len(incomplete) == 0 && recursionDepth < maxRecursionDepth {
			recursionDepth *= recursionMultiplier
		}
	}
	return out, nil
}

// fetchBlocksRecursively gets the blocks from recursionDepth ancestor tipsets
// starting from baseCid.
func (gsf *GraphSyncFetcher) fetchBlocksRecursively(ctx context.Context, baseCid cid.Cid, recursionDepth int, gss GraphsyncSession) error {
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

	return gss.Request(ctx, []cid.Cid{baseCid}, selector, false)
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
