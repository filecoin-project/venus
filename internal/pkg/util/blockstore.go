package util

import (
	"context"
	"github.com/filecoin-project/venus/internal/pkg/fork/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"go.opencensus.io/trace"
)

func CopyBlockstore(ctx context.Context, from, to blockstore.Blockstore) error {
	ctx, span := trace.StartSpan(ctx, "copyBlockstore")
	defer span.End()
	cids, err := from.AllKeysChan(ctx)
	if err != nil {
		return err
	}

	// TODO: should probably expose better methods on the blockstore for this operation
	var blks []blocks.Block
	for c := range cids {
		b, err := from.Get(c)
		if err != nil {
			return err
		}

		blks = append(blks, b)
	}

	if err := to.PutMany(blks); err != nil {
		return err
	}

	return nil
}
