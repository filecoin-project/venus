package state

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

// NewFromString creates a new VM state based on the contents of the string.
func NewFromString(t *testing.T, state string, store cbor.IpldStore) *State {
	panic("resurrect")
}

// MustCommit flushes the StateTree or panics if it can't.
func MustCommit(st State) cid.Cid {
	cid, err := st.Commit(context.Background())
	if err != nil {
		panic(err)
	}
	return cid
}

// MustGetActor gets the actor or panics if it can't.
func MustGetActor(st State, a address.Address) (*actor.Actor, bool) {
	actor, found, err := st.GetActor(context.Background(), a)
	if err != nil {
		panic(err)
	}
	return actor, found
}

// MustSetActor sets the actor or panics if it can't.
func MustSetActor(st State, address address.Address, actor *actor.Actor) cid.Cid {
	err := st.SetActor(context.Background(), address, actor)
	if err != nil {
		panic(err)
	}
	return MustCommit(st)
}
