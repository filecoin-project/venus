package state

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/pkg/types"
)

// NewFromString sets a state tree based on an int.
//
// TODO: we could avoid this if write a test cborStore that can map test cids to test states.
func NewFromString(t *testing.T, s string, store cbor.IpldStore) *State {
	tree, err := NewState(store, StateTreeVersion0)
	require.NoError(t, err)
	strAddr, err := address.NewSecp256k1Address([]byte(s))
	fmt.Printf("strAddr: %s\n", strAddr)
	require.NoError(t, err)
	err = tree.SetActor(context.Background(), strAddr, &types.Actor{})
	require.NoError(t, err)
	return tree
}

// MustCommit flushes the state or panics if it can't.
func MustCommit(st State) cid.Cid {
	cid, err := st.Flush(context.Background())
	if err != nil {
		panic(err)
	}
	return cid
}

// MustGetActor gets the actor or panics if it can't.
func MustGetActor(st State, a address.Address) (*types.Actor, bool) {
	actor, found, err := st.GetActor(context.Background(), a)
	if err != nil {
		panic(err)
	}
	return actor, found
}

// MustSetActor sets the actor or panics if it can't.
func MustSetActor(st State, address address.Address, actor *types.Actor) cid.Cid {
	err := st.SetActor(context.Background(), address, actor)
	if err != nil {
		panic(err)
	}
	return MustCommit(st)
}
