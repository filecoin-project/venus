package net

import (
	"context"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipld/go-ipld-prime"
	ipldfree "github.com/ipld/go-ipld-prime/impl/free"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/libp2p/go-libp2p-core/peer"
)

// interface conformance check
var _ Fetcher = (*GraphSyncFetcher)(nil)

// GraphExchange is an interface wrapper to Graphsync so it can be stubbed in
// unit testing
type GraphExchange interface {
	Request(ctx context.Context, p peer.ID, root ipld.Link, selector ipld.Node) (<-chan graphsync.ResponseProgress, <-chan error)
}

// GraphSyncFetcher is used to fetch data over the network.  It is implemented with
// using a Graphsync exchange to fetch tipsets recursively
type GraphSyncFetcher struct {
	exchange  GraphExchange
	validator consensus.BlockSyntaxValidator
	store     bstore.Blockstore
	ssb       selector.SelectorSpecBuilder
}

// NewGraphSyncFetcher returns a GraphsyncFetcher wired up to the input Graphsync exchange and attached local
// blockservice for reloading blocks in memory once they are returned
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

// FetchTipSets gets Tipsets starting from the given tipset key and continuing till
// the done function returns true or errors
//
// For now FetchTipSets operates in two parts:
// 1. It fetches relevant blocks through Graphsync, which writes them to the block store
// 2. It reads them from the block store and validates there syntax as blocks
// and constructs a tipset
// This does have a potentially unwanted side effect of writing blocks to the block store
// that later don't validate (bitswap actually does this as well)
// In the future, the blocks will be validated directly through graphsync as
// go-filecoin migrates to the same IPLD library used by go-graphsync (go-ipld-prime)
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
	baseCid := cids[0]
	recursionDepth := 1
	for !isDone {
		cidSets, err := gsf.fetchBlocksRecursively(ctx, baseCid, from, recursionDepth)
		if err != nil {
			return nil, err
		}
		for _, cids := range cidSets {
			ts, err := gsf.loadTipsetFromCids(ctx, cids)
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
			baseCid = cids[0]
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

// fetchBlocksRecursively gets multiple sets of parent blocks starting from a baseCid, up to
// the given recursion depth parameter
func (gsf *GraphSyncFetcher) fetchBlocksRecursively(ctx context.Context, baseCid cid.Cid, from peer.ID, recursionDepth int) ([][]cid.Cid, error) {

	// recursive selector to fetch n sets of parent blocks
	// starting from block matching base cid:
	//   - fetch all parent blocks
	//   - with exactly the first parent block, repeat again for it's parents
	//   - continue up to recursion depth
	selector := gsf.ssb.ExploreRecursive(recursionDepth, gsf.ssb.ExploreFields(func(efsb selector.ExploreFieldsSpecBuilder) {
		efsb.Insert("parents", gsf.ssb.ExploreUnion(
			gsf.ssb.ExploreAll(gsf.ssb.Matcher()),
			gsf.ssb.ExploreIndex(0, gsf.ssb.ExploreRecursiveEdge()),
		))
	})).Node()
	cidMaps := make([]map[cid.Cid]struct{}, recursionDepth)

	responseChan, errChan := gsf.exchange.Request(ctx, from, cidlink.Link{Cid: baseCid}, selector)
	for responseChan != nil || errChan != nil {
		select {
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			return nil, err
		case response, ok := <-responseChan:
			if !ok {
				responseChan = nil
				continue
			}
			// Paths returned in a traversal are of the format "","parent","parent/0",
			// "parent/0/parent", "parent/0/parent/0", and so on. Each time the number of
			// segments in the path goes up by two, we traverse to the next set of parents
			// We care about blocks starting with the first set of parents --
			// those at "parents/n" so we calculate the depth by dividing the
			// number of path segments by two and subtracting one, skipping over
			// "" & "parent", which refer to the block at baseCid
			depth := (len(response.Path.Segments()) / 2) - 1
			if depth < 0 {
				continue
			}
			cidMap := cidMaps[depth]
			if cidMap == nil {
				cidMap = make(map[cid.Cid]struct{})
				cidMaps[depth] = cidMap
			}
			asCidLink := response.LastBlock.Link.(cidlink.Link)
			cidMap[asCidLink.Cid] = struct{}{}
		}
	}
	return cidMapstoCidSets(cidMaps), nil
}

func cidMapstoCidSets(cidMaps []map[cid.Cid]struct{}) [][]cid.Cid {
	cidSets := make([][]cid.Cid, len(cidMaps))
	for i, cidMap := range cidMaps {
		cidSet := make([]cid.Cid, 0, len(cidMap))
		for c := range cidMap {
			cidSet = append(cidSet, c)
		}
		cidSets[i] = cidSet
	}
	return cidSets
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
