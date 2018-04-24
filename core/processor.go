package core

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"
)

// Processor is the signature a function used to process blocks.
type Processor func(ctx context.Context, blk *types.Block, st types.StateTree) ([]*types.MessageReceipt, error)

// ProcessBlock is the entrypoint for validating the state transitions
// of the messages in a block. When we receive a new block from the
// network ProcessBlock applies the block's messages to the beginning
// state tree ensuring that all transitions are valid, accumulating
// changes in the state tree, and returning the message receipts.
//
// ProcessBlock returns an error if it hits a message the application
// of which would result in an invalid state transition (eg, a
// message to transfer value from an unknown account). ProcessBlock
// can return one of three kinds of errors (see ApplyMessage: fault
// error, permanent error, temporary error). For the purposes of
// block validation the caller probably doesn't care if the error
// was temporary or permanent; either way the block has a bad
// message and should be thrown out. Caller should always differentiate
// a fault error as it signals something Very Bad has happened
// (eg, disk corruption).
//
// To be clear about intent: if ProcessBlock returns an ApplyError
// it is signaling that the message should not have been included
// in the block. If no error is returned this means that the
// message was applied, BUT SUCCESSFUL APPLICATION DOES NOT
// NECESSARILY MEAN THE IMPLIED CALL SUCCEEDED OR SENDER INTENT
// WAS REALIZED. It just means that the transition if any was
// valid. For example, a message that errors out in the VM
// will in many cases be successfully applied even though an
// error was thrown causing any state changes to be rolled back.
// See comments on ApplyMessage for specific intent.
//
// TODO Note that this method is also temporarily the entrypoint for
// block generation when mining: we use it to attempt to apply
// the messages in the message pool. However since ProcessBlock
// is all or nothing this isn't really approriate: the mining
// worker wants to use ApplyMessage directly to attempt to apply
// the message, then do something specific with the message in the
// case of a temporary or permanent error; ProcessBlock doesn't make
// that distinction. This'll be addressed in a follow up.
func ProcessBlock(ctx context.Context, blk *types.Block, st types.StateTree) ([]*types.MessageReceipt, error) {
	var receipts []*types.MessageReceipt
	emptyReceipts := []*types.MessageReceipt{}

	for _, msg := range blk.Messages {
		r, err := ApplyMessage(ctx, st, msg)
		// If the message should not have been in the block, bail.
		// TODO: handle faults appropriately at a higher level.
		if IsFault(err) || IsApplyErrorPermanent(err) || IsApplyErrorTemporary(err) {
			return emptyReceipts, err
		} else if err != nil {
			return emptyReceipts, faultErrorWrap(err, "someone is a bad progarmmer: must be a fault, perm, or temp error")
		}

		// TODO fritz check caller assumptions about receipts.
		receipts = append(receipts, r)
	}
	return receipts, nil
}

// ApplyMessage attempts to apply a message to a state tree. It is the
// sole driver of state tree transitions in the system. Both block
// validation and mining use this function and we should treat any changes
// to it with extreme care.
//
// If ApplyMessage returns no error then the message was successfully applied
// to the state tree: it did not result in any invalid transitions. As you will see
// below, this does not necessarily mean that the message "succeeded" for some
// senses of "succeeded". We choose therefore to say the message was or was not
// successfully applied.
//
// If ApplyMessage returns an error then the message would've resulted in
// an invalid state transition -- it was not successfully applied. When
// ApplyMessage returns an error one of three predicates will be true:
//   - IsFault(err): a system fault occurred (corrupt disk, violated precondition,
//     etc). This is Bad. Caller should stop doing whatever they are doing and get a doctor.
//     No guarantees are made about the state of the state tree.
//   - IsApplyErrorPermanent: the message was not applied and is unlikely
//     to *ever* be successfully applied (equivalently, it is unlikely to
//     ever result in a valid state transition). For example, the message might
//     attempt to transfer negative value. The message should probably be discarded.
//     All state tree mutations will have been reverted.
//   - IsApplyErrorTemporary: the message was not applied though it is
//     *possible* but not certain that the message may become applyable in
//     the future (eg, nonce is too high). The state was reverted.
//
// Please carefully consider the following intent with respect to messages.
// The intentions span the following concerns:
//   - whether the message was successfully applied: if not don't include it
//     in a block. If so inc sender's nonce and include it in a block.
//   - whether the message might be successfully applied at a later time
//     (IsApplyErrorTemporary) vs not (IsApplyErrorPermanent). If the caller
//     is the mining code it could remove permanently unapplyable messages from
//     the message pool but keep temporarily unapplyable messages around to try
//     applying to a future block.
//   - whether to keep or revert state: should we keep or revert state changes
//     caused by the message and its callees? We always revert state changes
//     from unapplyable messages. We might or might not revert changes from
//     applyable messages.
//
// Specific intentions include:
//   - fault errors: immediately return to the caller no matter what
//   - nonce too low: permanently unapplyable (don't include, revert changes, discard)
// TODO: if we have a re-order of the chain the message with nonce too low could
//       become applyable. Except that we already have a message with that nonce.
//       Maybe give this more careful consideration?
//   - nonce too high: temporarily unapplyable (don't include, revert, keep in pool)
//   - sender account exists but insufficient funds: successfully applied
//       (include it in the block but revert its changes). This an explicit choice
//       to make failing transfers not replayable (just like a bank transfer is not
//       replayable).
//   - sender account does not exist: temporarily unapplyable (don't include, revert,
//       keep in pool). There could be an account-creating message forthcoming.
//       (TODO this is only true while we don't have nonce checking; nonce checking
//       will cover this case in the future)
//   - send to self: permanently unapplyable (don't include in a block, revert changes,
//       discard)
//   - transfer negative value: permanently unapplyable (as above)
//   - all other vmerrors: successfully applied! Include in the block and
//       revert changes. Necessarily all vm errors that are not faults are
//       revert errors.
//   - everything else: successfully applied (include, keep changes)
//
// TODO need to document what the contract is with callees of ApplyMessage,
// for example squintinig at this perhaps:
//   - ApplyMessage loads the to and from actor and sets the result on the tree
//   - changes should accumulate in the actor in callees
//   - no callee including actor invocations should set to/from actor on the
//     state tree (see above: changes should accumulate)
//   - no callee should get a different pointer to the to/from actors
//       (we assume the pointer we have accumulates all the changes)
//   - if a callee creates another actor it must set it on the state tree
//   - ApplyMessage and VMContext.Send() are the only things that should call
//     Send() -- all the user-actor logic goes in ApplyMessage and all the
//     actor-actor logic goes in VMContext.Send
func ApplyMessage(ctx context.Context, st types.StateTree, msg *types.Message) (*types.MessageReceipt, error) {
	ss := st.Snapshot()
	r, err := attemptApplyMessage(ctx, st, msg)
	if IsFault(err) {
		return r, err
	} else if shouldRevert(err) {
		st.RevertTo(ss)
	} else if err != nil {
		return nil, newFaultError("someone is a bad programmer: only return revert and fault errors")
	}

	// Reject invalid state transitions.
	if err == errAccountNotFound || err == errNonceTooHigh {
		return nil, applyErrorTemporaryWrapf(err, "apply message failed")
	} else if err == errSelfSend || err == errNonceTooLow || err == ErrCannotTransferNegativeValue {
		return nil, applyErrorPermanentWrapf(err, "apply message failed")
	} else if err != nil { // nolint: megacheck
		// Do nothing. All other vm errors are ok: the state was rolled back
		// above but we applied the message successfully. This intentionally
		// includes errInsufficientFunds because we don't want the message
		// to be replayable.
	}

	// At this point we consider the message successfully applied so inc
	// the nonce.
	fromActor, err := st.GetActor(ctx, msg.From)
	if err != nil {
		return nil, faultErrorWrap(err, "couldn't load from actor")
	}
	fromActor.IncNonce()
	if err := st.SetActor(ctx, msg.From, fromActor); err != nil {
		return nil, faultErrorWrap(err, "could not set from actor after inc nonce")
	}

	return r, nil
}

var (
	// These errors are only to be used by ApplyMessage; they shouldn't be
	// used in any other context as they are an implementation detail.
	errAccountNotFound = newRevertError("account not found")
	errNonceTooHigh    = newRevertError("nonce too high")
	errNonceTooLow     = newRevertError("nonce too low")
	// TODO we'll eventually handle sending to self.
	errSelfSend = newRevertError("cannot send to self")
)

// attemptApplyMessage encapsulates the work of trying to apply the message in order
// to make ApplyMessage more readable. The distinction is that attemptApplyMessage
// should deal with trying got apply the message to the state tree whereas
// ApplyMessage should deal with any side effects and how it should be presented
// to the caller. attemptApplyMessage should only be called from ApplyMessage.
func attemptApplyMessage(ctx context.Context, st types.StateTree, msg *types.Message) (*types.MessageReceipt, error) {
	fromActor, err := st.GetActor(ctx, msg.From)
	if types.IsActorNotFoundError(err) {
		return nil, errAccountNotFound
	} else if err != nil {
		return nil, faultErrorWrapf(err, "failed to get From actor %s", msg.From)
	}

	if msg.From == msg.To {
		// TODO: handle this
		return nil, errSelfSend
	}

	toActor, err := st.GetOrCreateActor(ctx, msg.To, func() (*types.Actor, error) {
		a, err := NewAccountActor(nil)
		if err != nil {
			// Note: we're inside a closure; any error will be wrapped below.
			return nil, err
		}
		return a, st.SetActor(ctx, msg.To, a)
	})
	if err != nil {
		return nil, faultErrorWrap(err, "failed to get To actor")
	}

	c, err := msg.Cid()
	if err != nil {
		return nil, faultErrorWrap(err, "failed to get CID from the message")
	}

	if msg.Nonce < fromActor.Nonce {
		return nil, errNonceTooLow
	}
	if msg.Nonce > fromActor.Nonce {
		return nil, errNonceTooHigh
	}

	ret, exitCode, vmErr := Send(ctx, fromActor, toActor, msg, st)

	if IsFault(vmErr) {
		return nil, vmErr
	}

	if err := st.SetActor(ctx, msg.From, fromActor); err != nil {
		return nil, faultErrorWrap(err, "could not set from actor after send")
	}
	if err := st.SetActor(ctx, msg.To, toActor); err != nil {
		return nil, faultErrorWrap(err, "could not set to actor after send")
	}

	vmErrString := ""
	if vmErr != nil {
		vmErrString = vmErr.Error()
	}

	return types.NewMessageReceipt(c, exitCode, vmErrString, ret), vmErr
}
