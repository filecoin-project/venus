package msg

import (
	"context"
	"fmt"
	"sync"

	"testing"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestSend(t *testing.T) {
	t.Parallel()

	t.Run("send message enqueues and calls publish", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		repo, w, chainStore, msgPool := setupSendTest(require)
		addr, err := wallet.NewAddress(w)
		require.NoError(err)

		publishCalled := false
		publish := func(topic string, data []byte) error {
			assert.Equal(Topic, topic)
			publishCalled = true
			return nil
		}

		s := NewSender(repo, w, chainStore, msgPool, publish)
		require.Equal(0, len(msgPool.Pending()))
		_, err = s.Send(context.Background(), addr, addr, types.NewAttoFILFromFIL(uint64(2)), types.NewGasPrice(0), types.NewGasUnits(0), "")
		require.NoError(err)
		assert.Equal(1, len(msgPool.Pending()))
		assert.True(publishCalled)
	})

	t.Run("send message avoids nonce race", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		repo, w, chainStore, msgPool := setupSendTest(require)
		addr, err := wallet.NewAddress(w)
		require.NoError(err)
		nopPublish := func(string, []byte) error { return nil }
		s := NewSender(repo, w, chainStore, msgPool, nopPublish)

		var wg sync.WaitGroup
		addTwentyMessages := func(batch int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_, err := s.Send(ctx, addr, addr, types.NewZeroAttoFIL(), types.NewGasPrice(0), types.NewGasUnits(0), fmt.Sprintf("%d-%d", batch, i), []byte{})
				require.NoError(err)
			}
		}

		// Add messages concurrently.
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go addTwentyMessages(i)
		}

		wg.Wait()

		assert.Equal(60, len(msgPool.Pending()))

		// Expect none of the messages to have the same nonce.
		nonces := map[uint64]bool{}
		for _, message := range msgPool.Pending() {
			_, found := nonces[uint64(message.Nonce)]
			require.False(found)
			nonces[uint64(message.Nonce)] = true
		}
	})

}

func TestNextNonce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("account does not exist, should return zero", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		store := hamt.NewCborStore()
		st := state.NewEmptyStateTree(store)

		address := address.NewForTestGetter()()

		n, err := nextNonce(ctx, st, core.NewMessagePool(), address)
		assert.NoError(err)
		assert.Equal(uint64(0), n)
	})

	t.Run("account exists but wrong type", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		store := hamt.NewCborStore()
		st := state.NewEmptyStateTree(store)

		address := address.NewForTestGetter()()
		actor, err := storagemarket.NewActor()
		assert.NoError(err)
		_ = state.MustSetActor(st, address, actor)

		_, err = nextNonce(ctx, st, core.NewMessagePool(), address)
		assert.Error(err)
		assert.Contains(err.Error(), "account or empty")
	})

	t.Run("account exists, gets correct value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		store := hamt.NewCborStore()
		st := state.NewEmptyStateTree(store)
		address := address.NewForTestGetter()()
		actor, err := account.NewActor(types.NewAttoFILFromFIL(0))
		assert.NoError(err)
		actor.Nonce = 42
		state.MustSetActor(st, address, actor)

		nonce, err := nextNonce(ctx, st, core.NewMessagePool(), address)
		assert.NoError(err)
		assert.Equal(uint64(42), nonce)
	})

	t.Run("gets nonce from highest message pool value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		store := hamt.NewCborStore()
		st := state.NewEmptyStateTree(store)
		mp := core.NewMessagePool()
		addr := mockSigner.Addresses[0]
		actor, err := account.NewActor(types.NewAttoFILFromFIL(0))
		assert.NoError(err)
		actor.Nonce = 2
		state.MustSetActor(st, addr, actor)

		nonce, err := nextNonce(ctx, st, mp, addr)
		assert.NoError(err)
		assert.Equal(uint64(2), nonce)

		msg := types.NewMessage(addr, address.TestAddress, nonce, nil, "", []byte{})
		smsg := testhelpers.MustSign(mockSigner, msg)
		core.MustAdd(mp, smsg...)

		nonce, err = nextNonce(ctx, st, mp, addr)
		assert.NoError(err)
		assert.Equal(uint64(3), nonce)

		msg = types.NewMessage(addr, address.TestAddress, nonce, nil, "", []byte{})
		smsg = testhelpers.MustSign(mockSigner, msg)
		core.MustAdd(mp, smsg...)

		nonce, err = nextNonce(ctx, st, mp, addr)
		assert.NoError(err)
		assert.Equal(uint64(4), nonce)
	})
}

func setupSendTest(require *require.Assertions) (repo.Repo, *wallet.Wallet, *chain.DefaultStore, *core.MessagePool) {
	d := requireCommonDeps(require)
	return d.repo, d.wallet, d.chainStore, core.NewMessagePool()
}
