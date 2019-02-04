package msg

import (
	"context"
	"fmt"
	"sync"

	"testing"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend(t *testing.T) {
	t.Parallel()

	t.Run("send message enqueues and calls publish", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		repo, w, chainStore, msgPool := SetupSendTest(require)
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

		repo, w, chainStore, msgPool := SetupSendTest(require)
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

	t.Run("account does not exist should return zero", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		_, _, chainStore, msgPool := SetupSendTest(require)

		noActorAddress := address.NewForTestGetter()()
		n, err := nextNonce(ctx, chainStore, msgPool, noActorAddress)
		require.NoError(err)
		assert.Equal(uint64(0), n)
	})

	t.Run("account exists, largest value is in message pool", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		_, w, chainStore, msgPool := SetupSendTest(require)
		addr, err := wallet.NewAddress(w)
		require.NoError(err)

		msg := types.NewMessage(addr, addr, 0, nil, "foo", []byte{})
		msg.Nonce = 42
		smsg, err := types.NewSignedMessage(*msg, w, types.NewGasPrice(0), types.NewGasUnits(0))
		assert.NoError(err)
		core.MustAdd(msgPool, smsg)

		nonce, err := nextNonce(ctx, chainStore, msgPool, addr)
		assert.NoError(err)
		assert.Equal(uint64(43), nonce)
	})
}

func SetupSendTest(require *require.Assertions) (repo.Repo, *wallet.Wallet, *chain.DefaultStore, *core.MessagePool) {
	d := requireCommonDeps(require)
	return d.repo, d.wallet, d.chainStore, core.NewMessagePool()
}
