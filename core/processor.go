package core

import (
	"context"
	"fmt"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// Processor is the signature a function used to process blocks.
type Processor func(ctx context.Context, blk *types.Block, st types.StateTree) ([]*types.MessageReceipt, error)

// ProcessBlock takes a block and a state tree and applies the state
// transitions specified in the block on top of the state tree.
func ProcessBlock(ctx context.Context, blk *types.Block, st types.StateTree) ([]*types.MessageReceipt, error) {
	var receipts []*types.MessageReceipt

	for _, msg := range blk.Messages {
		receipt, err := ApplyMessage(ctx, st, msg)
		switch {
		default:
			return receipts, err
		case ShouldRevert(err):
			// TODO: revert state changes for this last message
			panic("TODO")
		case err == nil:
			receipts = append(receipts, receipt)
			// noop
		}
	}
	return receipts, nil
}

// ApplyMessage applies the state transition specified by the given
// message to the state tree.
func ApplyMessage(ctx context.Context, st types.StateTree, msg *types.Message) (*types.MessageReceipt, error) {
	if msg.From == msg.To {
		// TODO: handle this
		return nil, fmt.Errorf("unhandled: sending to self (%s)", msg.From)
	}

	fromActor, err := st.GetActor(ctx, msg.From)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get From actor %s", msg.From)
	}

	toActor, err := st.GetOrCreateActor(ctx, msg.To, func() (*types.Actor, error) {
		return NewAccountActor(nil)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get To actor")
	}

	c, err := msg.Cid()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get CID from the message")
	}

	ret, exitCode, err := Send(ctx, fromActor, toActor, msg, st)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send message")
	}

	if err := st.SetActor(ctx, msg.From, fromActor); err != nil {
		return nil, err
	}
	if err := st.SetActor(ctx, msg.To, toActor); err != nil {
		return nil, err
	}

	return types.NewMessageReceipt(c, exitCode, ret), nil
}
