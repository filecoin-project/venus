package msg

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

func TestSend(t *testing.T) {
	t.Parallel()

	t.Run("invalid message rejected", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		w, chainStore, msgPool := setupSendTest(require)
		addr := w.Addresses()[0]
		nopPublish := func(string, []byte) error { return nil }

		s := NewSender(w, chainStore, msgPool, nullValidator{rejectMessages: true}, nopPublish)
		_, err := s.Send(context.Background(), addr, addr, types.NewAttoFILFromFIL(2), types.NewGasPrice(0), types.NewGasUnits(0), "")
		assert.Errorf(err, "for testing")
	})

	t.Run("send message enqueues and calls publish", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		w, chainStore, msgPool := setupSendTest(require)
		addr := w.Addresses()[0]

		publishCalled := false
		publish := func(topic string, data []byte) error {
			assert.Equal(Topic, topic)
			publishCalled = true
			return nil
		}

		s := NewSender(w, chainStore, msgPool, nullValidator{}, publish)
		require.Equal(0, len(msgPool.Pending()))
		_, err := s.Send(context.Background(), addr, addr, types.NewAttoFILFromFIL(2), types.NewGasPrice(0), types.NewGasUnits(0), "")
		require.NoError(err)
		assert.Equal(1, len(msgPool.Pending()))
		assert.True(publishCalled)
	})

	t.Run("send message avoids nonce race", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		w, chainStore, msgPool := setupSendTest(require)
		addr := w.Addresses()[0]
		nopPublish := func(string, []byte) error { return nil }
		s := NewSender(w, chainStore, msgPool, nullValidator{}, nopPublish)

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

		n, err := nextNonce(ctx, st, core.NewMessagePool(testhelpers.NewTestBlockTimer(0)), address)
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

		_, err = nextNonce(ctx, st, core.NewMessagePool(testhelpers.NewTestBlockTimer(0)), address)
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

		nonce, err := nextNonce(ctx, st, core.NewMessagePool(testhelpers.NewTestBlockTimer(0)), address)
		assert.NoError(err)
		assert.Equal(uint64(42), nonce)
	})

	t.Run("gets nonce from highest message pool value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		store := hamt.NewCborStore()
		st := state.NewEmptyStateTree(store)
		mp := core.NewMessagePool(testhelpers.NewTestBlockTimer(0))
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

type nullValidator struct {
	rejectMessages bool
}

func (v nullValidator) Validate(ctx context.Context, msg *types.SignedMessage, fromActor *actor.Actor) error {
	if v.rejectMessages {
		return errors.New("rejected for testing")
	}
	return nil
}

func setupSendTest(require *require.Assertions) (*wallet.Wallet, *chain.DefaultStore, *core.MessagePool) {
	// Install an account actor in the genesis block.
	ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())[0]
	addr, err := ki.Address()
	require.NoError(err)
	genesis := consensus.MakeGenesisFunc(
		consensus.ActorAccount(addr, types.NewAttoFILFromFIL(100)),
	)

	d := requiredCommonDeps(require, genesis)

	// Install the key in the wallet for use in signing.
	err = d.wallet.Backends(wallet.DSBackendType)[0].(*wallet.DSBackend).ImportKey(&ki)
	require.NoError(err)
	return d.wallet, d.chainStore, core.NewMessagePool(testhelpers.NewTestBlockTimer(0))
}
