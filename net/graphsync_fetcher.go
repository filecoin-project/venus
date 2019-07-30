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
// a persistent bitswap session on a networked blockservice.
type GraphSyncFetcher struct {
	exchange  GraphExchange
	validator consensus.BlockSyntaxValidator
	bs        bstore.Blockstore
	ssb       selector.SelectorSpecBuilder
}

// NewGraphSyncFetcher returns a BitswapFetcher wired up to the input BlockService and a newly
// initialized persistent session of the block service.
func NewGraphSyncFetcher(ctx context.Context, exchange GraphExchange, blockstore bstore.Blockstore,
	bv consensus.BlockSyntaxValidator) *GraphSyncFetcher {
	gsf := &GraphSyncFetcher{
		bs:        blockstore,
		validator: bv,
		exchange:  exchange,
		ssb:       selector.NewSelectorSpecBuilder(ipldfree.NodeBuilder()),
	}
	return gsf
}

const maxRecursionDepth = 64
const recursionMultiplier = 4

// FetchTipSets gets Tipsets starting from the given tipset key and continuing till
// the done function returns true or errors
func (gsf *GraphSyncFetcher) FetchTipSets(ctx context.Context, tsKey types.TipSetKey, from peer.ID, done func(types.TipSet) (bool, error)) ([]types.TipSet, error) {
	cids := tsKey.ToSlice()
	err := gsf.GetSingleLayer(ctx, cids, from)
	if err != nil {
		return nil, err
	}
	startingTipset, err := gsf.LoadTipsetFromCids(ctx, cids)
	if err != nil {
		return nil, err
	}
	out := []types.TipSet{startingTipset}
	isDone, err := done(startingTipset)
	if err != nil {
		return nil, err
	}
	if !isDone {
		baseCid := cids[0]
		recursionDepth := 1
	makeRecursion:
		for {
			cidResponseLayers, err := gsf.GetBlocks(ctx, baseCid, from, recursionDepth)
			if err != nil {
				return nil, err
			}
			for _, cidResponseLayer := range cidResponseLayers {
				ts, err := gsf.LoadTipsetFromCids(ctx, cidResponseLayer)
				if err != nil {
					return nil, err
				}

				out = append(out, ts)
				isDone, err := done(ts)
				if err != nil {
					return nil, err
				}

				if isDone {
					break makeRecursion
				}
				baseCid = cidResponseLayer[0]
			}
			if recursionDepth < maxRecursionDepth {
				recursionDepth *= recursionMultiplier
			}
		}
	}
	return out, nil
}

// LoadTipsetFromCids loads and sanitizes an already downloaded TipSet
// from an array of cids
func (gsf *GraphSyncFetcher) LoadTipsetFromCids(ctx context.Context, cidResponseLayer []cid.Cid) (types.TipSet, error) {
	blockResponseLayer := make([]blocks.Block, 0, len(cidResponseLayer))
	for _, cid := range cidResponseLayer {
		block, err := gsf.bs.Get(cid)
		if err != nil {
			return types.UndefTipSet, err
		}
		blockResponseLayer = append(blockResponseLayer, block)
	}
	validatedBlocks, err := sanitizeBlocks(ctx, blockResponseLayer, gsf.validator)
	if err != nil {
		return types.UndefTipSet, err
	}

	ts, err := types.NewTipSet(validatedBlocks...)
	if err != nil {
		return types.UndefTipSet, err
	}

	return ts, nil
}

type resultSet struct {
	responseChan <-chan graphsync.ResponseProgress
	errChan      <-chan error
}

// GetSingleLayer requests a single layer of cids as individual bocks, fetching
// non-recursively
func (gsf *GraphSyncFetcher) GetSingleLayer(ctx context.Context, cids []cid.Cid, from peer.ID) error {
	selector := gsf.ssb.Matcher().Node()
	resultSets := make([]resultSet, 0, len(cids))
	for _, c := range cids {
		responseChan, errChan := gsf.exchange.Request(ctx, from, cidlink.Link{Cid: c}, selector)
		resultSets = append(resultSets, resultSet{responseChan, errChan})
	}
	for _, rs := range resultSets {
		for err := range rs.errChan {
			return err
		}
		for range rs.responseChan {
		}
	}
	return nil
}

// GetBlocks gets multiple layers of parent blocks starting from a baseCid, up to
// the given recursion depth parameter
func (gsf *GraphSyncFetcher) GetBlocks(ctx context.Context, baseCid cid.Cid, from peer.ID, recursionDepth int) ([][]cid.Cid, error) {

	selector := gsf.ssb.ExploreRecursive(recursionDepth, gsf.ssb.ExploreFields(func(efsb selector.ExploreFieldsSpecBuilder) {
		efsb.Insert("parents", gsf.ssb.ExploreUnion(
			gsf.ssb.ExploreAll(gsf.ssb.Matcher()),
			gsf.ssb.ExploreIndex(0, gsf.ssb.ExploreRecursiveEdge()),
		))
	})).Node()
	cidResponseLayers := make([][]cid.Cid, recursionDepth)
	hasCids := make([]map[cid.Cid]struct{}, recursionDepth)

	responseChan, errChan := gsf.exchange.Request(ctx, from, cidlink.Link{Cid: baseCid}, selector)
	for err := range errChan {
		return nil, err
	}
	for response := range responseChan {
		// we are only interested in specific blocks
		depth := (len(response.Path.Segments()) / 2) - 1
		if depth < 0 {
			continue
		}
		hasCidsLayer := hasCids[depth]
		if hasCidsLayer == nil {
			hasCidsLayer = make(map[cid.Cid]struct{})
			hasCids[depth] = hasCidsLayer
		}
		asCidLink, ok := response.LastBlock.Link.(cidlink.Link)
		if !ok {
			continue
		}
		cur := asCidLink.Cid
		_, ok = hasCidsLayer[cur]
		if ok {
			continue
		}
		hasCidsLayer[cur] = struct{}{}
		cidResponseLayers[depth] = append(cidResponseLayers[depth], cur)
	}
	return cidResponseLayers, nil
}
