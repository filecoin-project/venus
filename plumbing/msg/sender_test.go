package msg

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
	hamt "github.com/ipfs/go-hamt-ipld"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend(t *testing.T) {
	t.Parallel()

	t.Run("invalid message rejected", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		w, chainStore, cst := setupSendTest(require)
		addr := w.Addresses()[0]
		timer := testhelpers.NewTestMessagePoolAPI(1000)
		queue := core.NewMessageQueue()
		pool := core.NewMessagePool(timer, config.NewDefaultConfig().Mpool, testhelpers.NewMockMessagePoolValidator())
		nopPublish := func(string, []byte) error { return nil }

		s := NewSender(w, chainStore, cst, timer, queue, pool, nullValidator{rejectMessages: true}, nopPublish)
		_, err := s.Send(context.Background(), addr, addr, types.NewAttoFILFromFIL(2), types.NewGasPrice(0), types.NewGasUnits(0), "")
		assert.Errorf(err, "for testing")
	})

	t.Run("send message enqueues and calls publish", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		w, chainStore, cst := setupSendTest(require)
		addr := w.Addresses()[0]
		toAddr := address.NewForTestGetter()()
		timer := testhelpers.NewTestMessagePoolAPI(1000)
		queue := core.NewMessageQueue()
		pool := core.NewMessagePool(timer, config.NewDefaultConfig().Mpool, testhelpers.NewMockMessagePoolValidator())

		publishCalled := false
		publish := func(topic string, data []byte) error {
			assert.Equal(Topic, topic)
			publishCalled = true
			return nil
		}

		s := NewSender(w, chainStore, cst, timer, queue, pool, nullValidator{}, publish)
		require.Empty(queue.List(addr))
		require.Empty(pool.Pending())

		_, err := s.Send(context.Background(), addr, toAddr, types.NewZeroAttoFIL(), types.NewGasPrice(0), types.NewGasUnits(0), "")
		require.NoError(err)
		assert.Equal(uint64(1000), queue.List(addr)[0].Stamp)
		assert.Equal(1, len(pool.Pending()))
		assert.True(publishCalled)
	})

	t.Run("send message avoids nonce race", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		// number of messages to send
		msgCount := 20
		// number of of concurrent message sends
		sendConcurrent := 3
		// total messages sent == msgCount * sendConcurrent
		mpoolCfg := config.NewDefaultConfig().Mpool
		mpoolCfg.MaxPoolSize = msgCount * sendConcurrent

		w, chainStore, cst := setupSendTest(require)
		addr := w.Addresses()[0]
		toAddr := address.NewForTestGetter()()
		timer := testhelpers.NewTestMessagePoolAPI(1000)
		queue := core.NewMessageQueue()
		pool := core.NewMessagePool(timer, mpoolCfg, testhelpers.NewMockMessagePoolValidator())
		nopPublish := func(string, []byte) error { return nil }

		s := NewSender(w, chainStore, cst, timer, queue, pool, nullValidator{}, nopPublish)

		var wg sync.WaitGroup
		addTwentyMessages := func(batch int) {
			defer wg.Done()
			for i := 0; i < msgCount; i++ {
				_, err := s.Send(ctx, addr, toAddr, types.NewZeroAttoFIL(), types.NewGasPrice(0), types.NewGasUnits(0), fmt.Sprintf("%d-%d", batch, i), []byte{})
				require.NoError(err)
			}
		}

		// Add messages concurrently.
		for i := 0; i < sendConcurrent; i++ {
			wg.Add(1)
			go addTwentyMessages(i)
		}

		wg.Wait()

		assert.Equal(60, len(pool.Pending()))

		// Expect none of the messages to have the same nonce.
		nonces := map[uint64]bool{}
		for _, message := range pool.Pending() {
			_, found := nonces[uint64(message.Nonce)]
			require.False(found)
			nonces[uint64(message.Nonce)] = true
		}
	})
}

func TestNextNonce(t *testing.T) {
	t.Parallel()

	t.Run("account exists but wrong type", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		address := address.NewForTestGetter()()
		actor, err := storagemarket.NewActor()
		assert.NoError(err)

		_, err = nextNonce(actor, core.NewMessageQueue(), address)
		assert.Error(err)
		assert.Contains(err.Error(), "account or empty")
	})

	t.Run("account exists, gets correct value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		address := address.NewForTestGetter()()
		actor, err := account.NewActor(types.NewAttoFILFromFIL(0))
		assert.NoError(err)
		actor.Nonce = 42

		nonce, err := nextNonce(actor, core.NewMessageQueue(), address)
		assert.NoError(err)
		assert.Equal(uint64(42), nonce)
	})

	t.Run("gets nonce from highest message queue value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		outbox := core.NewMessageQueue()
		addr := mockSigner.Addresses[0]
		actor, err := account.NewActor(types.NewAttoFILFromFIL(0))
		assert.NoError(err)
		actor.Nonce = 2

		nonce, err := nextNonce(actor, outbox, addr)
		assert.NoError(err)
		assert.Equal(uint64(2), nonce)

		msg := types.NewMessage(addr, address.TestAddress, nonce, nil, "", []byte{})
		smsg := testhelpers.MustSign(mockSigner, msg)
		core.MustEnqueue(outbox, 100, smsg...)

		nonce, err = nextNonce(actor, outbox, addr)
		assert.NoError(err)
		assert.Equal(uint64(3), nonce)

		msg = types.NewMessage(addr, address.TestAddress, nonce, nil, "", []byte{})
		smsg = testhelpers.MustSign(mockSigner, msg)
		core.MustEnqueue(outbox, 100, smsg...)

		nonce, err = nextNonce(actor, outbox, addr)
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

func setupSendTest(require *require.Assertions) (*wallet.Wallet, *chain.DefaultStore, *hamt.CborIpldStore) {
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
	return d.wallet, d.chainStore, d.cst
}
