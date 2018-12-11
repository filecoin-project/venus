package message

import (
	"context"
	"fmt"
	"sync"

	hamt "gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	bserv "gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	"gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"
	"testing"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSendTest(require *require.Assertions) (repo.Repo, *wallet.Wallet, *chain.DefaultStore, *core.MessagePool) {
	r := repo.NewInMemoryRepo()

	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}
	chainStore, err := chain.Init(context.Background(), r, bs, cst, consensus.InitGenesis)
	require.NoError(err)

	backend, err := wallet.NewDSBackend(r.WalletDatastore())
	require.NoError(err)
	wallet := wallet.New(backend)

	msgPool := core.NewMessagePool()

	return r, wallet, chainStore, msgPool
}

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
		_, err = s.Send(context.Background(), addr, addr, types.NewAttoFILFromFIL(uint64(2)), "")
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
				_, err := s.Send(ctx, addr, addr, types.NewZeroAttoFIL(), fmt.Sprintf("%d-%d", batch, i), []byte{})
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

		_, _, chainStore, msgPool := setupSendTest(require)

		noActorAddress := address.NewForTestGetter()()
		n, err := nextNonce(ctx, chainStore, msgPool, noActorAddress)
		require.NoError(err)
		assert.Equal(uint64(0), n)
	})

	t.Run("account exists, largest value is in message pool", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		_, w, chainStore, msgPool := setupSendTest(require)
		addr, err := wallet.NewAddress(w)
		require.NoError(err)

		msg := types.NewMessage(addr, addr, 0, nil, "foo", []byte{})
		msg.Nonce = 42
		smsg, err := types.NewSignedMessage(*msg, w)
		assert.NoError(err)
		core.MustAdd(msgPool, smsg)

		nonce, err := nextNonce(ctx, chainStore, msgPool, addr)
		assert.NoError(err)
		assert.Equal(uint64(43), nonce)
	})
}

func TestGetAndMaybeSetDefaultSenderAddress(t *testing.T) {
	t.Parallel()

	t.Run("it returns the configured wallet default if it exists", func(t *testing.T) {
		require := require.New(t)
		repo, w, _, _ := setupSendTest(require)

		// generate a default address
		addrA, err := wallet.NewAddress(w)
		require.NoError(err)

		// load up the wallet with a few more addresses
		for i := 0; i < 10; i++ {
			wallet.NewAddress(w)
		}

		// configure a default
		repo.Config().Wallet.DefaultAddress = addrA

		addrB, err := GetAndMaybeSetDefaultSenderAddress(repo, w)
		require.NoError(err)
		require.Equal(addrA.String(), addrB.String())
	})

	t.Run("default is consistent if none configured", func(t *testing.T) {
		require := require.New(t)
		repo, w, _, _ := setupSendTest(require)

		// generate a few addresses
		// load up the wallet with a few more addresses
		addresses := []address.Address{}
		for i := 0; i < 10; i++ {
			a, err := wallet.NewAddress(w)
			require.NoError(err)
			addresses = append(addresses, a)
		}

		// remove existing wallet config
		repo.Config().Wallet = &config.WalletConfig{}

		expected, err := GetAndMaybeSetDefaultSenderAddress(repo, w)
		require.NoError(err)
		require.True(isInList(expected, addresses))
		for i := 0; i < 30; i++ {
			got, err := GetAndMaybeSetDefaultSenderAddress(repo, w)
			require.NoError(err)
			require.Equal(expected, got)
		}
	})
}

func isInList(needle address.Address, haystack []address.Address) bool {
	for _, a := range haystack {
		if a == needle {
			return true
		}
	}
	return false
}
