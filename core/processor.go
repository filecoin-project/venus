package core

import (
	"context"
	"fmt"
	"math/big"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/types"
)

// Processor is the signature a function used to process blocks.
type Processor func(ctx context.Context, blk *types.Block, st types.StateTree) error

// ProcessBlock takes a block and a state tree and applies the state
// transitions specified in the block on top of the state tree.
func ProcessBlock(ctx context.Context, blk *types.Block, st types.StateTree) error {
	for _, msg := range blk.Messages {
		err := ApplyMessage(ctx, st, msg)
		switch {
		default:
			return err
		case ShouldRevert(err):
			// TODO: revert state changes for this last message
			panic("TODO")
		case err == nil:
			// noop
		}
	}
	return nil
}

// ApplyMessage applies the state transition specified by the given
// message to the state tree.
func ApplyMessage(ctx context.Context, st types.StateTree, msg *types.Message) error {
	if msg.From() == msg.To() {
		// TODO: handle this
		return fmt.Errorf("unhandled: sending to self")
	}

	fromAct, err := st.GetActor(ctx, msg.From())
	if err != nil {
		return errors.Wrap(err, "failed to get From actor")
	}

	if fromAct.Balance.Cmp(msg.Value()) < 0 {
		return &revertErrorWrap{fmt.Errorf("not enough funds")}
	}

	fromAct.Balance.Sub(fromAct.Balance, msg.Value())

	toAct, err := st.GetActor(ctx, msg.To())
	switch err {
	default:
		return err
	case hamt.ErrNotFound:
		toAct = &types.Actor{Balance: big.NewInt(0)}
	case nil:
	}

	toAct.Balance.Add(toAct.Balance, msg.Value())

	if err := st.SetActor(ctx, msg.From(), fromAct); err != nil {
		return err
	}

	if err := st.SetActor(ctx, msg.To(), toAct); err != nil {
		return err
	}

	return nil
}
