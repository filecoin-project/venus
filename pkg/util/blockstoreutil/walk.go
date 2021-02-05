package blockstoreutil

/*
import (
	"bytes"
	"context"
	"fmt"
	"github.com/exascience/pargo/parallel"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	xerrors "github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"
	"sync"
)

func WalkSnapshot(ctx context.Context, bs bstore.blockstore, ts *block.TipSet, incRecentEpoch abi.ChainEpoch, inclRecentRoots abi.ChainEpoch) error {
	seen := NewSet()
	walked := NewSet()
	blocksToWalk := ts.Key().Cids()
	walkChain := func(blk cid.Cid) error {
		if !seen.Visit(blk) {
			return nil
		}

		data, err := bs.Get(blk)
		if err != nil {
			return xerrors.Errorf("getting block: %w", err)
		}

		var b block.BlockHeader
		if err := b.UnmarshalCBOR(bytes.NewReader(data.RawData())); err != nil {
			return xerrors.Errorf("unmarshaling block header (cid=%s): %w", blk, err)
		}

		var cids []cid.Cid
		fmt.Println("Height:", b.Height)
		if b.Height > 0 && b.Height > ts.EnsureHeight()-incRecentEpoch {
			for _, p := range b.Parents.Cids() {
				blocksToWalk = append(blocksToWalk, p)
			}
		} else {
			// include the genesis block
			cids = append(cids, b.Parents.Cids()...)
		}

		out := cids

		if b.Height == 0 || b.Height > ts.EnsureHeight()-inclRecentRoots {
			if walked.Visit(b.ParentStateRoot) {
				cids, err := recurseLinks(bs, walked, b.ParentStateRoot)
				if err != nil {
					return xerrors.Errorf("recursing genesis state failed: %w", err)
				}

				out = append(out, cids...)
			}
		}
		fmt.Println("cid to fetch ", len(out))
		var fns []func()
		for _, c := range out {
			if seen.Visit(c) {
				if c.Prefix().Codec != cid.DagCBOR {
					continue
				}
				fns = append(fns, func(cid2 cid.Cid) func() {
					return func() {
						bs.Get(c)
					}
				}(c))
			}
		}
		parallel.Do(fns...)
		return nil
	}

	for len(blocksToWalk) > 0 {
		next := blocksToWalk[0]
		blocksToWalk = blocksToWalk[1:]
		if err := walkChain(next); err != nil {
			return xerrors.Errorf("walk chain failed: %w", err)
		}
	}

	return nil
}

func recurseLinks(bs bstore.blockstore, walked *Set, root cid.Cid) ([]cid.Cid, error) {
	if root.Prefix().Codec != cid.DagCBOR {
		return []cid.Cid{root}, nil
	}

	data, err := bs.Get(root)
	if err != nil {
		return nil, xerrors.Errorf("recurse links get (%s) failed: %w", root, err)
	}

	var rerr error
	var wg sync.WaitGroup
	var lock sync.Mutex
	var out []cid.Cid
	err = cbg.ScanForLinks(bytes.NewReader(data.RawData()), func(c cid.Cid) {
		if rerr != nil {
			// No error return on ScanForLinks :(
			return
		}
		// traversed this already...
		if !walked.Visit(c) {
			return
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			pin, err := recurseLinks(bs, walked, c)
			if err != nil {
				rerr = err
			}
			lock.Lock()
			out = append(out, pin...)
			lock.Unlock()
		}()

	})
	wg.Wait()
	if err != nil {
		return nil, xerrors.Errorf("scanning for links failed: %w", err)
	}

	return out, rerr
}
*/
