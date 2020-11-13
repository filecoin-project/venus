package state

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-state-types/abi"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/account"
	init0 "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
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
	tree, err := NewStateWithBuiltinActor(t, store, StateTreeVersion0)
	require.NoError(t, err)

	//create account
	strAddr, err := address.NewSecp256k1Address([]byte(s))
	fmt.Printf("strAddr: %s\n", strAddr)
	require.NoError(t, err)

	//add a account for t3
	AddAccount(t, tree, store, strAddr)
	require.NoError(t, err)
	return tree
}

func NewStateWithBuiltinActor(t *testing.T, store cbor.IpldStore, ver StateTreeVersion) (*State, error) {
	ctx := context.TODO()
	tree, err := NewState(store, StateTreeVersion0)
	require.NoError(t, err)
	adtStore := &AdtStore{store}

	//create builtin init account
	emptyMap := adt.MakeEmptyMap(adtStore)
	emptyMapRoot, err := emptyMap.Root()
	require.NoError(t, err)
	initState := &init0.State{
		AddressMap:  emptyMapRoot,
		NextID:      20,
		NetworkName: "test-net",
	}

	initCodeID, err := store.Put(ctx, initState)
	require.NoError(t, err)
	initActor := &types.Actor{
		Code:       enccid.NewCid(builtin0.InitActorCodeID),
		Head:       enccid.NewCid(initCodeID),
		CallSeqNum: 0,
		Balance:    abi.TokenAmount{},
	}
	err = tree.SetActor(ctx, builtin0.InitActorAddr, initActor)
	require.NoError(t, err)
	return tree, nil
}

func AddAccount(t *testing.T, tree *State, store cbor.IpldStore, addr address.Address) {
	ctx := context.TODO()
	adtStore := &AdtStore{store}

	initActor, _, err := tree.GetActor(ctx, builtin0.InitActorAddr)
	require.NoError(t, err)
	initState := &init0.State{}
	err = adtStore.Get(ctx, initActor.Head.Cid, initState)
	require.NoError(t, err)
	//add a account for t3
	idAddr, err := initState.MapAddressToNewID(adtStore, addr)
	require.NoError(t, err)
	newInitStateId, err := store.Put(ctx, initState) //nolint
	require.NoError(t, err)
	initActor.Head = enccid.NewCid(newInitStateId)
	err = tree.SetActor(ctx, builtin0.InitActorAddr, initActor)
	require.NoError(t, err)

	emptyObject, err := store.Put(context.TODO(), []struct{}{})
	if err != nil {
		panic(err)
	}
	accountActor := &types.Actor{
		Code:       enccid.NewCid(builtin0.AccountActorCodeID),
		Head:       enccid.NewCid(emptyObject),
		CallSeqNum: 0,
		Balance:    abi.TokenAmount{},
	}
	err = tree.SetActor(ctx, idAddr, accountActor)
	require.NoError(t, err)

	//save t3 address
	accountState := &account.State{Address: addr}
	accountRoot, err := store.Put(context.TODO(), accountState)
	if err != nil {
		panic(err)
	}
	addrActor := &types.Actor{
		Code:       enccid.NewCid(builtin0.AccountActorCodeID),
		Head:       enccid.NewCid(accountRoot),
		CallSeqNum: 0,
		Balance:    abi.TokenAmount{},
	}
	err = tree.SetActor(context.Background(), addr, addrActor)
	require.NoError(t, err)
}

func UpdateAccount(t *testing.T, tree *State, addr address.Address, fn func(*types.Actor)) {
	ctx := context.TODO()
	actor, _, err := tree.GetActor(ctx, addr)
	require.NoError(t, err)
	fn(actor)
	err = tree.SetActor(context.Background(), addr, actor)
	require.NoError(t, err)
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
