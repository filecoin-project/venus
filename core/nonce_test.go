package core

import (
	"context"
	"testing"

	hamt "gx/ipfs/QmbwwhSsEcSPP4XfGumu6GMcuCLnCLVQAnp3mDxKuYNXJo/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
)

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
		actor, err := storagemarket.NewActor()
		assert.NoError(err)
		_ = state.MustSetActor(st, address, actor)

		_, err = NextNonce(ctx, st, mp, address)
		assert.Error(err)
		assert.Contains(err.Error(), "not an account or empty actor")
	})

	t.Run("account exists, gets correct value", func(t *testing.T) {
		assert := assert.New(t)
		store := hamt.NewCborStore()
		st := state.NewEmptyStateTree(store)
		mp := NewMessagePool()
		address := types.NewAddressForTestGetter()()
		actor, err := account.NewActor(types.NewAttoFILFromFIL(0))
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
		addr := mockSigner.Addresses[0]
		actor, err := account.NewActor(types.NewAttoFILFromFIL(0))
		assert.NoError(err)
		actor.Nonce = 2
		state.MustSetActor(st, addr, actor)

		nonce, err := NextNonce(ctx, st, mp, addr)
		assert.NoError(err)
		assert.Equal(uint64(2), nonce)

		msg := types.NewMessage(addr, address.TestAddress, nonce, nil, "", []byte{})
		smsg := MustSign(mockSigner, msg)
		MustAdd(mp, smsg...)

		nonce, err = NextNonce(ctx, st, mp, addr)
		assert.NoError(err)
		assert.Equal(uint64(3), nonce)

		msg = types.NewMessage(addr, address.TestAddress, nonce, nil, "", []byte{})
		smsg = MustSign(mockSigner, msg)
		MustAdd(mp, smsg...)

		nonce, err = NextNonce(ctx, st, mp, addr)
		assert.NoError(err)
		assert.Equal(uint64(4), nonce)
	})
}
