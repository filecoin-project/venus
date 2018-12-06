package core

import (
	"context"
	"fmt"

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
	actor, err := st.GetActor(ctx, address)
	if err != nil {
		return 0, err
	}
	if actor.Code.Defined() && !actor.Code.Equals(types.AccountActorCodeCid) {
		return 0, xerrors.New("actor not an account or empty actor")
	}

	nonce := uint64(actor.Nonce)

	// TODO: consider what if anything to do if there's a gap with
	// what's in the pool.
	nonceFromMsgPool, found := LargestNonce(mp, address)

	x := uint64(actor.Nonce)
	if found && nonceFromMsgPool >= x {
		x = nonceFromMsgPool + 1
	}

	fmt.Printf("%-5s | %-35s | %-20s | foundSomethingInPool=%v, currentActorNonce=%-d, whatWeWillUse=%d\n", "gf", "      NextNonce", "", found, nonce, x)

	if found && nonceFromMsgPool >= nonce {
		nonce = nonceFromMsgPool + 1
	}

	return nonce, nil
}
