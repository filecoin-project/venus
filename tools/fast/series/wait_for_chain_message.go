package series

import (
	"context"
	"fmt"
	"io"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/ipfs/go-cid"
)

// MsgInfo contains the BlockCid for the message MsgCid
type MsgInfo struct {
	BlockCid cid.Cid
	MsgCid   cid.Cid
}

// MsgSearchFn is the function signature used to find a message
type MsgSearchFn func(context.Context, *fast.Filecoin, *types.SignedMessage) (bool, error)

// WaitForChainMessage iterates over the chain until the provided function `fn` returns true.
func WaitForChainMessage(ctx context.Context, node *fast.Filecoin, fn MsgSearchFn) (*MsgInfo, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-CtxSleepDelay(ctx):
			dec, err := node.ChainLs(ctx)
			if err != nil {
				return nil, err
			}

			for dec.More() {
				var blks []block.Block
				err := dec.Decode(&blks)
				if err != nil {
					if err == io.EOF {
						break
					}

					return nil, err
				}

				if msgInfo, err := findMessageInBlockSlice(ctx, node, blks, fn); err == nil {
					return msgInfo, nil
				}
			}
		}
	}
}

func findMessageInBlockSlice(ctx context.Context, node *fast.Filecoin, blks []block.Block, fn MsgSearchFn) (*MsgInfo, error) {
	for _, blk := range blks {
		msgs, err := node.ShowMessages(ctx, blk.Messages.SecpRoot)
		if err != nil {
			return nil, err
		}

		for _, msg := range msgs {
			found, err := fn(ctx, node, msg)
			if err != nil {
				return nil, err
			}

			if found {
				blockCid := blk.Cid()
				msgCid, _ := msg.Cid()

				return &MsgInfo{
					BlockCid: blockCid,
					MsgCid:   msgCid,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("Message not found")
}
