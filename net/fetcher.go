package net

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
	blocks "github.com/ipfs/go-block-format"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	"github.com/ipfs/go-graphsync/ipldbridge"
	gsnet "github.com/ipfs/go-graphsync/network"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	ipldp "github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	selector "github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
)

// Fetcher defines an interface that may be used to fetch data from the network.
type Fetcher interface {
	// FetchTipSets will only fetch TipSets that evaluate to `false` when passed to `done`,
	// this includes the provided `ts`. The TipSet that evaluates to true when
	// passed to `done` will be in the returned slice. The returns slice of TipSets is in Traversal order.
	FetchTipSets(ctx context.Context, tsKey types.TipSetKey, from peer.ID, done func(ts types.TipSet) (bool, error)) ([]types.TipSet, error)
}

// GraphSyncFetcher is used to fetch data over the network.  It is implemented with
// a persistent bitswap session on a networked blockservice.
type GraphSyncFetcher struct {
	session   *bserv.Session
	gs        *graphsync.GraphSync
	validator consensus.BlockSyntaxValidator
	bridge    ipldbridge.IPLDBridge
	bs        bstore.Blockstore
}

// NewGraphSyncFetcher returns a BitswapFetcher wired up to the input BlockService and a newly
// initialized persistent session of the block service.
func NewGraphSyncFetcher(ctx context.Context, network gsnet.GraphSyncNetwork, bridge ipldbridge.IPLDBridge, blockstore bstore.Blockstore,
	bv consensus.BlockSyntaxValidator) *GraphSyncFetcher {
	gsf := &GraphSyncFetcher{
		bs:        blockstore,
		validator: bv,
		bridge:    bridge,
	}
	gsf.gs = graphsync.New(ctx, network, bridge, gsf.Loader, gsf.Storer)
	return gsf
}

func (gsf *GraphSyncFetcher) Loader(lnk ipldp.Link, lnkCtx ipldp.LinkContext) (io.Reader, error) {
	asCidLink, ok := lnk.(cidlink.Link)
	if !ok {
		return nil, fmt.Errorf("Unsupported Link Type")
	}
	block, err := gsf.bs.Get(asCidLink.Cid)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(block.RawData()), nil
}

func (gsf *GraphSyncFetcher) Storer(lnkCtx ipldp.LinkContext) (io.Writer, ipldp.StoreCommitter, error) {
	var buffer bytes.Buffer
	committer := func(lnk ipldp.Link) error {
		asCidLink, ok := lnk.(cidlink.Link)
		if !ok {
			return fmt.Errorf("Unsupported Link Type")
		}
		block, err := blocks.NewBlockWithCid(buffer.Bytes(), asCidLink.Cid)
		if err != nil {
			return err
		}
		return gsf.bs.Put(block)
	}
	return &buffer, committer, nil
}

const maxRecursionDepth = 64
const recursionMultiplier = 4

func (gsf *GraphSyncFetcher) FetchTipSets(ctx context.Context, tsKey types.TipSetKey, from peer.ID, done func(types.TipSet) (bool, error)) ([]types.TipSet, error) {
	var out []types.TipSet
	recursionDepth := 1
	cur := tsKey
makeRecursion:
	for {
		blockResponseLayers, err := gsf.GetBlocks(ctx, cur.ToSlice(), from, recursionDepth)
		if err != nil {
			return nil, err
		}
		for _, blockResponseLayer := range blockResponseLayers {
			validatedBlocks, err := sanitizeBlocks(ctx, blockResponseLayer, gsf.validator)
			if err != nil {
				return nil, err
			}

			ts, err := types.NewTipSet(validatedBlocks...)
			if err != nil {
				return nil, err
			}

			isDone, err := done(ts)
			if err != nil {
				return nil, err
			}

			if isDone {
				break makeRecursion
			}

			// this is perhaps unneccesary based on graphsync validation, but worth double
			// checking we got the tipset we expected!
			if !ts.Key().Equals(cur) {
				return nil, fmt.Errorf("Tipset doesn't match expected")
			}

			cur, err = ts.Parents()

			if err != nil {
				return nil, err
			}
			out = append(out, ts)
		}
		if recursionDepth < maxRecursionDepth {
			recursionDepth *= recursionMultiplier
		}
	}

	return out, nil
}

type gsResultSet struct {
	responseChan <-chan graphsync.ResponseProgress
	errorChan    <-chan error
}

func (gsf *GraphSyncFetcher) GetBlocks(ctx context.Context, cids []cid.Cid, from peer.ID, recursionDepth int) ([][]blocks.Block, error) {

	selector, err := gsf.bridge.BuildSelector(func(ssb selector.SelectorSpecBuilder) selector.SelectorSpec {
		return ssb.ExploreRecursive(recursionDepth, ssb.ExploreFields(func(efsb selector.ExploreFieldsSpecBuilder) {
			efsb.Insert("Parents", ssb.ExploreUnion(
				ssb.ExploreAll(ssb.Matcher()),
				ssb.ExploreIndex(0, ssb.ExploreRecursiveEdge()),
			))
		}))
	})
	if err != nil {
		return nil, err
	}

	blockResponseLayers := make([][]blocks.Block, recursionDepth)
	hasBlock := make([]map[cid.Cid]struct{}, recursionDepth)
	resultSets := make([]gsResultSet, 0, recursionDepth)
	for _, cur := range cids {
		responseChan, errChan := gsf.gs.Request(ctx, from, cidlink.Link{Cid: cur}, selector)
		resultSets = append(resultSets, gsResultSet{responseChan, errChan})
	}

	for _, resultSet := range resultSets {
		responseChan := resultSet.responseChan
		errChan := resultSet.errorChan
		for err := range errChan {
			return nil, err
		}
		for response := range responseChan {
			// we are only interested in specific blocks
			depth := (len(response.Path.Segments()) / 2)
			hasBlockLayer := hasBlock[depth]
			if hasBlockLayer == nil {
				hasBlockLayer = make(map[cid.Cid]struct{})
				hasBlock[depth] = hasBlockLayer
			}
			cur := response.LastBlock.Link.(cidlink.Link).Cid
			_, ok := hasBlockLayer[cur]
			if !ok {
				hasBlockLayer[cur] = struct{}{}
				block, err := gsf.bs.Get(cur)
				if err != nil {
					return nil, err
				}
				blockResponseLayers[depth] = append(blockResponseLayers[depth], block)
			}
		}
	}
	return blockResponseLayers, nil
}

// BitswapFetcher is used to fetch data over the network.  It is implemented with
// a persistent bitswap session on a networked blockservice.
type BitswapFetcher struct {
	// session is a bitswap session that enables efficient transfer.
	session   *bserv.Session
	validator consensus.BlockSyntaxValidator
}

// NewBitswapFetcher returns a BitswapFetcher wired up to the input BlockService and a newly
// initialized persistent session of the block service.
func NewBitswapFetcher(ctx context.Context, bsrv bserv.BlockService, bv consensus.BlockSyntaxValidator) *BitswapFetcher {
	return &BitswapFetcher{
		session:   bserv.NewSession(ctx, bsrv),
		validator: bv,
	}
}

// FetchTipSets fetchs the tipset at `tsKey` from the network using the fetchers bitswap session.
func (bsf *BitswapFetcher) FetchTipSets(ctx context.Context, tsKey types.TipSetKey, from peer.ID, done func(types.TipSet) (bool, error)) ([]types.TipSet, error) {
	var out []types.TipSet
	cur := tsKey
	for {
		res, err := bsf.GetBlocks(ctx, cur.ToSlice())
		if err != nil {
			return nil, err
		}

		ts, err := types.NewTipSet(res...)
		if err != nil {
			return nil, err
		}

		out = append(out, ts)
		ok, err := done(ts)
		if err != nil {
			return nil, err
		}
		if ok {
			break
		}

		cur, err = ts.Parents()
		if err != nil {
			return nil, err
		}

	}

	return out, nil

}

// GetBlocks fetches the blocks with the given cids from the network using the
// BitswapFetcher's bitswap session.
func (bsf *BitswapFetcher) GetBlocks(ctx context.Context, cids []cid.Cid) ([]*types.Block, error) {
	var unsanitized []blocks.Block
	for b := range bsf.session.GetBlocks(ctx, cids) {
		unsanitized = append(unsanitized, b)
	}

	if len(unsanitized) < len(cids) {
		var err error
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = errors.Wrap(ctxErr, "failed to fetch all requested blocks")
		} else {
			err = errors.New("failed to fetch all requested blocks")
		}
		return nil, err
	}

	blocks, err := sanitizeBlocks(ctx, unsanitized, bsf.validator)
	if err != nil {
		return nil, err
	}
	return blocks, nil
}

func sanitizeBlocks(ctx context.Context, unsanitized []blocks.Block, validator consensus.BlockSyntaxValidator) ([]*types.Block, error) {
	var blocks []*types.Block
	for _, u := range unsanitized {
		block, err := types.DecodeBlock(u.RawData())
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("fetched data (cid %s) was not a block", u.Cid().String()))
		}

		// reject blocks that are syntactically invalid.
		if err := validator.ValidateSyntax(ctx, block); err != nil {
			continue
		}

		blocks = append(blocks, block)
	}
	return blocks, nil
}
