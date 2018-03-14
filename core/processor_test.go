package core

import (
	"context"
	"math/big"
	"testing"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	hamt "gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(fakeActorStorage{})
}

func requireMakeStateTree(require *require.Assertions, cst *hamt.CborIpldStore, acts map[types.Address]*types.Actor) (*cid.Cid, types.StateTree) {
	ctx := context.Background()
	t := types.NewEmptyStateTree(cst)

	for addr, act := range acts {
		err := t.SetActor(ctx, addr, act)
		require.NoError(err)
	}

	c, err := t.Flush(ctx)
	require.NoError(err)

	return c, t
}

func requireNewAccountActor(require *require.Assertions, value *big.Int) *types.Actor {
	act, err := NewAccountActor(value)
	require.NoError(err)
	return act
}

func TestProcessBlockSuccess(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	addr1, addr2 := newAddress(), newAddress()
	act1 := requireNewAccountActor(require, big.NewInt(10000))
	stCid, st := requireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
	})
	msg := types.NewMessage(addr1, addr2, big.NewInt(550), "", nil)
	blk := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.Message{msg},
	}
	receipts, err := ProcessBlock(ctx, blk, st)
	assert.NoError(err)
	assert.Len(receipts, 1)

	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)
	expAct1, expAct2 := requireNewAccountActor(require, big.NewInt(10000-550)), requireNewAccountActor(require, big.NewInt(550))
	expAct1.IncNonce()
	expStCid, _ := requireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: expAct1,
		addr2: expAct2,
	})
	assert.True(expStCid.Equals(gotStCid))
}

type fakeActorStorage struct{ Changed bool }
type fakeActor struct{}

var _ ExecutableActor = (*fakeActor)(nil)

var fakeActorExports = Exports{
	"foo": &FunctionSignature{
		Params: nil,
		Return: nil,
	},
}

// Exports returns the list of fakeActor exported functions.
func (ma *fakeActor) Exports() Exports {
	return fakeActorExports
}

// Foo sets a bit inside fakeActor's storage and returns a
// revert error.
func (ma *fakeActor) Foo(ctx *VMContext) (uint8, error) {
	fastore := &fakeActorStorage{}
	_, err := WithStorage(ctx, fastore, func() (interface{}, error) {
		fastore.Changed = true
		return nil, nil
	})
	if err != nil {
		panic(err.Error())
	}
	return 1, newRevertError("boom")
}

func requireNewFakeActor(require *require.Assertions, codeCid *cid.Cid) *types.Actor {
	storageBytes, err := MarshalStorage(&fakeActorStorage{})
	require.NoError(err)
	return types.NewActorWithMemory(codeCid, big.NewInt(100), storageBytes)
}

func TestProcessBlockVMErrors(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	BuiltinActors[fakeActorCodeCid.KeyString()] = &fakeActor{}
	defer func() {
		delete(BuiltinActors, fakeActorCodeCid.KeyString())
	}()

	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2 := newAddress(), newAddress()
	act1, act2 := requireNewFakeActor(require, fakeActorCodeCid), requireNewFakeActor(require, fakeActorCodeCid)
	stCid, st := requireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
		addr2: act2,
	})
	msg := types.NewMessage(addr1, addr2, nil, "foo", nil)
	blk := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.Message{msg},
	}

	// The "foo" message will cause a vm error and
	// we're going to check four things...
	receipts, err := ProcessBlock(ctx, blk, st)

	// 1. That a VM error is not a message failure (err).
	assert.NoError(err)

	// 2. That the VM error is faithfully recorded.
	assert.Len(receipts, 1)
	assert.Contains(receipts[0].Error, "boom")

	// 3 & 4. That on VM error the state is rolled back and nonce is inc'd.
	expectedAct1, expectedAct2 := requireNewFakeActor(require, fakeActorCodeCid), requireNewFakeActor(require, fakeActorCodeCid)
	expectedAct1.IncNonce()
	expectedStCid, _ := requireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: expectedAct1,
		addr2: expectedAct2,
	})
	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)
	assert.True(expectedStCid.Equals(gotStCid))
}
