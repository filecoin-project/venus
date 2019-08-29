package core_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/core"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestOutbox(t *testing.T) {
	tf.UnitTest(t)

	t.Run("invalid message rejected", func(t *testing.T) {
		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		queue := core.NewMessageQueue()
		publisher := &core.MockPublisher{}
		provider := core.NewFakeProvider(t)
		bcast := true

		ob := core.NewOutbox(w, core.FakeValidator{RejectMessages: true}, queue, publisher, core.NullPolicy{}, provider, provider)

		cid, err := ob.Send(context.Background(), sender, sender, types.NewAttoFILFromFIL(2), types.NewGasPrice(0), types.NewGasUnits(0), bcast, "")
		assert.Errorf(t, err, "for testing")
		assert.False(t, cid.Defined())
	})

	t.Run("send message enqueues and calls Publish, but respects bcast flag for broadcasting", func(t *testing.T) {
		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		toAddr := address.NewForTestGetter()()
		queue := core.NewMessageQueue()
		publisher := &core.MockPublisher{}
		provider := core.NewFakeProvider(t)

		head := provider.BuildOneOn(types.UndefTipSet, func(b *chain.BlockBuilder) {
			b.IncHeight(1000)
		})
		actr, _ := account.NewActor(types.ZeroAttoFIL)
		actr.Nonce = 42
		provider.SetHeadAndActor(t, head.Key(), sender, actr)

		ob := core.NewOutbox(w, core.FakeValidator{}, queue, publisher, core.NullPolicy{}, provider, provider)
		require.Empty(t, queue.List(sender))
		require.Nil(t, publisher.Message)

		testCases := []struct {
			bcast  bool
			nonce  types.Uint64
			height int
		}{{true, actr.Nonce, 1000}, {false, actr.Nonce + 1, 1000}}

		for _, test := range testCases {
			_, err := ob.Send(context.Background(), sender, toAddr, types.ZeroAttoFIL, types.NewGasPrice(0), types.NewGasUnits(0), test.bcast, "")
			require.NoError(t, err)
			assert.Equal(t, uint64(test.height), queue.List(sender)[0].Stamp)
			assert.NotNil(t, publisher.Message)
			assert.Equal(t, test.nonce, publisher.Message.Nonce)
			assert.Equal(t, uint64(test.height), publisher.Height)
			assert.Equal(t, test.bcast, publisher.Bcast)
		}

	})
	t.Run("send message avoids nonce race", func(t *testing.T) {
		ctx := context.Background()
		msgCount := 20      // number of messages to send
		sendConcurrent := 3 // number of of concurrent message sends

		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		toAddr := address.NewForTestGetter()()
		queue := core.NewMessageQueue()
		publisher := &core.MockPublisher{}
		provider := core.NewFakeProvider(t)
		bcast := true

		head := provider.BuildOneOn(types.UndefTipSet, func(b *chain.BlockBuilder) {
			b.IncHeight(1000)
		})
		actr, _ := account.NewActor(types.ZeroAttoFIL)
		actr.Nonce = 42
		provider.SetHeadAndActor(t, head.Key(), sender, actr)

		s := core.NewOutbox(w, core.FakeValidator{}, queue, publisher, core.NullPolicy{}, provider, provider)

		var wg sync.WaitGroup
		addTwentyMessages := func(batch int) {
			defer wg.Done()
			for i := 0; i < msgCount; i++ {
				method := fmt.Sprintf("%d-%d", batch, i)
				_, err := s.Send(ctx, sender, toAddr, types.ZeroAttoFIL, types.NewGasPrice(0), types.NewGasUnits(0), bcast, method, []byte{})
				require.NoError(t, err)
			}
		}

		// Add messages concurrently.
		for i := 0; i < sendConcurrent; i++ {
			wg.Add(1)
			go addTwentyMessages(i)
		}
		wg.Wait()

		enqueued := queue.List(sender)
		assert.Equal(t, 60, len(enqueued))

		// Expect the nonces to be distinct and contiguous
		nonces := map[uint64]bool{}
		for _, message := range enqueued {
			assert.Equal(t, uint64(1000), message.Stamp)
			_, found := nonces[uint64(message.Msg.Nonce)]
			require.False(t, found)
			nonces[uint64(message.Msg.Nonce)] = true
		}

		for i := 0; i < 60; i++ {
			assert.True(t, nonces[uint64(actr.Nonce)+uint64(i)])

		}
	})

	t.Run("fails with non-account actor", func(t *testing.T) {
		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		toAddr := address.NewForTestGetter()()
		queue := core.NewMessageQueue()
		publisher := &core.MockPublisher{}
		provider := core.NewFakeProvider(t)

		head := provider.NewGenesis()
		actr := storagemarket.NewActor() // Not an account actor
		provider.SetHeadAndActor(t, head.Key(), sender, actr)

		ob := core.NewOutbox(w, core.FakeValidator{}, queue, publisher, core.NullPolicy{}, provider, provider)

		_, err := ob.Send(context.Background(), sender, toAddr, types.ZeroAttoFIL, types.NewGasPrice(0), types.NewGasUnits(0), true, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "account or empty")
	})
}
