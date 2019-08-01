package net

import (
	"context"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipld/go-ipld-prime"
	ipldfree "github.com/ipld/go-ipld-prime/impl/free"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
)

// interface conformance check
var _ Fetcher = (*GraphSyncFetcher)(nil)

// GraphExchange is an interface wrapper to Graphsync so it can be stubbed in
// unit testing
type GraphExchange interface {
	Request(ctx context.Context, p peer.ID, root ipld.Link, selector ipld.Node) (<-chan graphsync.ResponseProgress, <-chan error)
}

// GraphSyncFetcher is used to fetch data over the network.  It is implemented
// using a Graphsync exchange to fetch tipsets recursively
type GraphSyncFetcher struct {
	exchange  GraphExchange
	validator consensus.BlockSyntaxValidator
	store     bstore.Blockstore
	ssb       selector.SelectorSpecBuilder
}

// NewGraphSyncFetcher returns a GraphsyncFetcher wired up to the input Graphsync exchange and
// attached local blockservice for reloading blocks in memory once they are returned
func NewGraphSyncFetcher(ctx context.Context, exchange GraphExchange, blockstore bstore.Blockstore,
	bv consensus.BlockSyntaxValidator) *GraphSyncFetcher {
	gsf := &GraphSyncFetcher{
		store:     blockstore,
		validator: bv,
		exchange:  exchange,
		ssb:       selector.NewSelectorSpecBuilder(ipldfree.NodeBuilder()),
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
func (gsf *GraphSyncFetcher) FetchTipSets(ctx context.Context, tsKey types.TipSetKey, from peer.ID, done func(types.TipSet) (bool, error)) ([]types.TipSet, error) {
	cids := tsKey.ToSlice()
	err := gsf.fetchBlocks(ctx, cids, from)
	if err != nil {
		return nil, err
	}
	startingTipset, err := gsf.loadTipsetFromCids(ctx, cids)
	if err != nil {
		return nil, err
	}
	out := []types.TipSet{startingTipset}
	isDone, err := done(startingTipset)
	if err != nil {
		return nil, err
	}
	recursionDepth := 1
	ts := startingTipset
	for !isDone {
		// Because a graphsync query always starts from a single CID,
		// we fetch tipsets starting from the first block in the last tipset and
		// recursively getting sets of parents
		err := gsf.fetchBlocksRecursively(ctx, ts.At(0).Cid(), from, recursionDepth)
		if err != nil {
			return nil, err
		}
		for i := 0; i < recursionDepth; i++ {
			tsKey, err := ts.Parents()
			if err != nil {
				return nil, err
			}
			cids := tsKey.ToSlice()
			ts, err = gsf.loadTipsetFromCids(ctx, cids)
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
		if recursionDepth < maxRecursionDepth {
			recursionDepth *= recursionMultiplier
		}
	}
	return out, nil
}

// fetchBlocks requests a single set of cids as individual bocks, fetching
// non-recursively
func (gsf *GraphSyncFetcher) fetchBlocks(ctx context.Context, cids []cid.Cid, from peer.ID) error {
	selector := gsf.ssb.Matcher().Node()
	errChans := make([]<-chan error, 0, len(cids))
	for _, c := range cids {
		_, errChan := gsf.exchange.Request(ctx, from, cidlink.Link{Cid: c}, selector)
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
func (gsf *GraphSyncFetcher) fetchBlocksRecursively(ctx context.Context, baseCid cid.Cid, from peer.ID, recursionDepth int) error {

	// recursive selector to fetch n sets of parent blocks
	// starting from block matching base cid:
	//   - fetch all parent blocks
	//   - with exactly the first parent block, repeat again for its parents
	//   - continue up to recursion depth
	selector := gsf.ssb.ExploreRecursive(recursionDepth, gsf.ssb.ExploreFields(func(efsb selector.ExploreFieldsSpecBuilder) {
		efsb.Insert("parents", gsf.ssb.ExploreUnion(
			gsf.ssb.ExploreAll(gsf.ssb.Matcher()),
			gsf.ssb.ExploreIndex(0, gsf.ssb.ExploreRecursiveEdge()),
		))
	})).Node()

	_, errChan := gsf.exchange.Request(ctx, from, cidlink.Link{Cid: baseCid}, selector)
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
