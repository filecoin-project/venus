package core

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/types"
)

// Processor is the signature a function used to process blocks.
type Processor func(ctx context.Context, blk *types.Block, st types.StateTree) ([]*types.MessageReceipt, error)

// ProcessBlock takes a block and a state tree and applies the state
// transitions specified in the block on top of the state tree. If
// any of the messages fail it returns an error. It does not roll the
// state back in case of failure (consistent with the ethereum behavior;
// caller is expected to just drop the state tree). Note that "failure"
// here means an invalid state transition such as insufficient funds,
// wrong nonce, etc. Valid messages that trigger a VM error do not
// constitute "failures" (with one exception; see ApplyMessage).
//
// This function is the main entrypoint for validating messages in
// new blocks that we see.
func ProcessBlock(ctx context.Context, blk *types.Block, st types.StateTree) ([]*types.MessageReceipt, error) {
	var receipts []*types.MessageReceipt

	for _, msg := range blk.Messages {
		receipt, err := ApplyMessage(ctx, st, msg)
		switch {
		default:
			// We saw a real failure to process a message. Bail.
			return nil, err
		case IsFault(err):
			// TODO: don't crash here, return an error that
			// stops whatever higher level thing is using ProcessBlock.
			panic(fmt.Sprintf("System fault: %s", err.Error()))
		case err == nil:
			receipts = append(receipts, receipt)
			// noop
		}
	}
	return receipts, nil
}

// ApplyMessage applies the message to the state tree. This function
// returns an error if the message fails. Failures include insufficient
// funds, wrong nonce, etc. With one exception a VM error does not
// constitute a message failure, so ApplyMessage does not return an
// error on vm error. When a vm error occurs ApplyMessage rolls
// state back to just before the vm invocation but after the nonce
// is inc'd and you can find the error text in Receipt.VMError.
//
// If ApplyMessage returns an error the caller should probalby roll
// the state back (or work with a new fresh copy).
//
// The one exception to vm-errors-are-not-message-failures is a transfer
// of funds error (eg, insufficient funds): these vm errors are treated
// as failures (ApplyMessage will return an error).
//
// If ApplyMessage returns an error for which IsFault(err) is true
// the system is in a badly broken state and the caller should stop
// whatever it is trying to do and get a doctor.
//
// ApplyMessage is used both to validate messages in newly seen blocks
// as well as to drive transitions of the state tree when assembling
// a block for mining.
func ApplyMessage(ctx context.Context, st types.StateTree, msg *types.Message) (*types.MessageReceipt, error) {
	if msg.From == msg.To {
		// TODO: handle this
		return nil, fmt.Errorf("unhandled: sending to self (%s)", msg.From)
	}

	fromActor, err := st.GetActor(ctx, msg.From)
	if err != nil {
		return nil, faultErrorWrapf(err, "failed to get From actor %s", msg.From)
	}

	toActor, err := st.GetOrCreateActor(ctx, msg.To, func() (*types.Actor, error) {
		return NewAccountActor(nil)
	})
	if err != nil {
		return nil, faultErrorWrap(err, "failed to get To actor")
	}

	c, err := msg.Cid()
	if err != nil {
		return nil, faultErrorWrap(err, "failed to get CID from the message")
	}

	// TODO(fritz) Turn on nonce checking in a separate change. Change was getting too big.
	// if msg.Nonce != fromActor.Nonce {
	// 	return nil, fmt.Errorf("message nonce %v not equal to required nonce %v", msg.Nonce, fromActor.Nonce)
	// }
	fromActor.IncNonce()
	if err := st.SetActor(ctx, msg.From, fromActor); err != nil {
		return nil, faultErrorWrap(err, "could not set from actor")
	}

	// TODO(fritz) Uncomment once this lands:
	// https://github.com/ipfs/go-hamt-ipld/pull/2
	// snapshot := st.Snapshot()

	ret, exitCode, vmErr := Send(ctx, fromActor, toActor, msg, st)

	if IsFault(vmErr) {
		return nil, vmErr
	} else if shouldRevert(vmErr) {
		// TODO(fritz) Uncomment once this lands:
		// https://github.com/ipfs/go-hamt-ipld/pull/2
		// st.RevertTo(snapshot)

		// Only a few VM errors constitute message failures (eg, insufficient funds).
		if isFailureVMError(vmErr) {
			return nil, vmErr
		}
		// Anything else is fine.
	} else {
		if err := st.SetActor(ctx, msg.From, fromActor); err != nil {
			return nil, faultErrorWrap(err, "could not set from actor after send")
		}
		if err := st.SetActor(ctx, msg.To, toActor); err != nil {
			return nil, faultErrorWrap(err, "could not set to actor after send")
		}
	}

	vmErrString := ""
	if vmErr != nil {
		vmErrString = vmErr.Error()
	}
	return types.NewMessageReceipt(c, exitCode, vmErrString, ret), nil
}
