package core

import (
	"context"
	"testing"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	hamt "gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountActorCborMarshaling(t *testing.T) {
	t.Run("CBOR decode(encode(Actor)) == identity(Actor)", func(t *testing.T) {
		require := require.New(t)

		preEncode, _ := NewAccountActor(types.NewTokenAmount(100))
		out, err := cbor.DumpObject(preEncode)
		require.NoError(err)

		var postDecode types.Actor
		err = cbor.DecodeInto(out, &postDecode)
		require.NoError(err)

		c1, _ := preEncode.Cid()
		require.NoError(err)

		c2, _ := postDecode.Cid()
		require.NoError(err)

		types.AssertCidsEqual(assert.New(t), c1, c2)
	})
}

func TestNextNonce(t *testing.T) {
	ctx := context.Background()

	t.Run("account does not exist", func(t *testing.T) {
		assert := assert.New(t)
		store := hamt.NewCborStore()
		st := state.NewEmptyStateTree(store)
		mp := NewMessagePool()

		address := types.NewAddressForTestGetter()()

		_, err := NextNonce(ctx, st, mp, address)
		assert.Error(err)
		assert.Contains(err.Error(), "not found")
	})

	t.Run("account exists but wrong type", func(t *testing.T) {
		assert := assert.New(t)
		store := hamt.NewCborStore()
		st := state.NewEmptyStateTree(store)
		mp := NewMessagePool()

		address := types.NewAddressForTestGetter()()
		actor, err := NewStorageMarketActor()
		assert.NoError(err)
		_ = state.MustSetActor(st, address, actor)

		_, err = NextNonce(ctx, st, mp, address)
		assert.Error(err)
		assert.Contains(err.Error(), "not an account actor")
	})

	t.Run("account exists, gets correct value", func(t *testing.T) {
		assert := assert.New(t)
		store := hamt.NewCborStore()
		st := state.NewEmptyStateTree(store)
		mp := NewMessagePool()
		address := types.NewAddressForTestGetter()()
		actor, err := NewAccountActor(types.NewTokenAmount(0))
		assert.NoError(err)
		actor.Nonce = 42
		state.MustSetActor(st, address, actor)

		nonce, err := NextNonce(ctx, st, mp, address)
		assert.NoError(err)
		assert.Equal(uint64(42), nonce)
	})

	t.Run("gets nonce from highest message pool value", func(t *testing.T) {
		assert := assert.New(t)
		store := hamt.NewCborStore()
		st := state.NewEmptyStateTree(store)
		mp := NewMessagePool()
		address := types.NewAddressForTestGetter()()
		actor, err := NewAccountActor(types.NewTokenAmount(0))
		assert.NoError(err)
		actor.Nonce = 2
		state.MustSetActor(st, address, actor)

		nonce, err := NextNonce(ctx, st, mp, address)
		assert.NoError(err)
		assert.Equal(uint64(2), nonce)

		msg := types.NewMessage(address, TestAddress, nonce, nil, "", []byte{})
		mp.Add(msg)

		nonce, err = NextNonce(ctx, st, mp, address)
		assert.NoError(err)
		assert.Equal(uint64(3), nonce)

		msg = types.NewMessage(address, TestAddress, nonce, nil, "", []byte{})
		mp.Add(msg)

		nonce, err = NextNonce(ctx, st, mp, address)
		assert.NoError(err)
		assert.Equal(uint64(4), nonce)
	})
}
