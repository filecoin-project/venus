package blockstoreutil

import (
	"bytes"
	"context"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	cbg "github.com/whyrusleeping/cbor-gen"
	"go.opencensus.io/trace"
)

func CopyBlockstore(ctx context.Context, from, to Blockstore) error {
	ctx, span := trace.StartSpan(ctx, "copyBlockstore")
	defer span.End()
	cids, err := from.AllKeysChan(ctx)
	if err != nil {
		return err
	}

	// TODO: should probably expose better methods on the blockstore for this operation
	var blks []blocks.Block
	for c := range cids {
		b, err := from.Get(ctx, c)
		if err != nil {
			return err
		}

		blks = append(blks, b)
	}

	return to.PutMany(ctx, blks)
}

func linksForObj(blk blocks.Block, cb func(cid.Cid)) error {
	switch blk.Cid().Prefix().Codec {
	case cid.DagCBOR:
		err := cbg.ScanForLinks(bytes.NewReader(blk.RawData()), cb)
		if err != nil {
			return fmt.Errorf("cbg.ScanForLinks: %v", err)
		}
		return nil
	case cid.Raw:
		// We implicitly have all children of raw blocks.
		return nil
	default:
		return fmt.Errorf("vm flush copy method only supports dag cbor")
	}
}

func CopyParticial(ctx context.Context, from, to Blockstore, root cid.Cid) error {
	ctx, span := trace.StartSpan(ctx, "vm.Copy") // nolint
	defer span.End()

	var numBlocks int
	var totalCopySize int

	const batchSize = 128
	const bufCount = 3
	freeBufs := make(chan []blocks.Block, bufCount)
	toFlush := make(chan []blocks.Block, bufCount)
	for i := 0; i < bufCount; i++ {
		freeBufs <- make([]blocks.Block, 0, batchSize)
	}

	errFlushChan := make(chan error)

	go func() {
		for b := range toFlush {
			if err := to.PutMany(ctx, b); err != nil {
				close(freeBufs)
				errFlushChan <- fmt.Errorf("batch put in copy: %v", err)
				return
			}
			freeBufs <- b[:0]
		}
		close(errFlushChan)
		close(freeBufs)
	}()

	var batch = <-freeBufs
	batchCp := func(blk blocks.Block) error {
		numBlocks++
		totalCopySize += len(blk.RawData())

		batch = append(batch, blk)

		if len(batch) >= batchSize {
			toFlush <- batch
			var ok bool
			batch, ok = <-freeBufs
			if !ok {
				return <-errFlushChan
			}
		}
		return nil
	}

	if err := copyRec(ctx, from, to, root, batchCp); err != nil {
		return fmt.Errorf("copyRec: %v", err)
	}

	if len(batch) > 0 {
		toFlush <- batch
	}
	close(toFlush)        // close the toFlush triggering the loop to end
	err := <-errFlushChan // get error out or get nil if it was closed
	if err != nil {
		return err
	}

	span.AddAttributes(
		trace.Int64Attribute("numBlocks", int64(numBlocks)),
		trace.Int64Attribute("copySize", int64(totalCopySize)),
	)
	return nil
}

func copyRec(ctx context.Context, from, to Blockstore, root cid.Cid, cp func(blocks.Block) error) error {
	if root.Prefix().MhType == 0 {
		// identity cid, skip
		return nil
	}

	blk, err := from.Get(ctx, root)
	if err != nil {
		return fmt.Errorf("get %s failed: %v", root, err)
	}

	var lerr error
	err = linksForObj(blk, func(link cid.Cid) {
		if lerr != nil {
			// Theres no erorr return on linksForObj callback :(
			return
		}

		prefix := link.Prefix()
		if prefix.Codec == cid.FilCommitmentSealed || prefix.Codec == cid.FilCommitmentUnsealed {
			return
		}

		// We always have blocks inlined into CIDs, but we may not have their children.
		if prefix.MhType == mh.IDENTITY {
			// Unless the inlined block has no children.
			if prefix.Codec == cid.Raw {
				return
			}
		} else {
			// If we have an object, we already have its children, skip the object.
			has, err := to.Has(ctx, link)
			if err != nil {
				lerr = fmt.Errorf("has: %v", err)
				return
			}
			if has {
				return
			}
		}

		if err := copyRec(ctx, from, to, link, cp); err != nil {
			lerr = err
			return
		}
	})
	if err != nil {
		return fmt.Errorf("linksForObj (%x): %v", blk.RawData(), err)
	}
	if lerr != nil {
		return lerr
	}

	if err := cp(blk); err != nil {
		return fmt.Errorf("copy: %v", err)
	}
	return nil
}
