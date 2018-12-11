package core

import (
	"context"

	xerrors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// NextNonce returns the next nonce for the account actor based on its memory.
// Depending on the context, this may or may not be sufficient to select a
// nonce for a message. See node.NextNonce if you want to select a nonce
// based on the state of the node (not just on the state of the actor).
func NextNonce(ctx context.Context, st state.Tree, mp *MessagePool, address address.Address) (uint64, error) {
	nonce := uint64(0)

	// Do the message pool check first: the address may not have an actor
	// on chain yet but might have a bunch of messages in the message pool.
	// TODO: consider what if anything to do if there's a gap with
	// what's in the pool.
	largestInPool, found := LargestNonce(mp, address)
	if found {
		nonce = largestInPool + 1
	}

	actor, err := st.GetActor(ctx, address)
	if state.IsActorNotFoundError(err) {
		return nonce, nil
	} else if err != nil {
		return 0, err
	}
	if actor.Code.Defined() && !actor.Code.Equals(types.AccountActorCodeCid) {
		return 0, xerrors.New("actor not an account or empty actor")
	}

	actorNonce := uint64(actor.Nonce)
	if actorNonce > nonce {
		nonce = actorNonce
	}

	return nonce, nil
}
