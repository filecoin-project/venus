package message_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/journal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

func newOutboxTestJournal(t *testing.T) journal.Writer {
	return journal.NewInMemoryJournal(t, clock.NewFake(time.Unix(1234567890, 0))).Topic("outbox")
}

func TestOutbox(t *testing.T) {
	tf.UnitTest(t)

	t.Run("invalid message rejected", func(t *testing.T) {
		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		queue := message.NewQueue()
		publisher := &message.MockPublisher{}
		provider := message.NewFakeProvider(t)
		bcast := true

		ob := message.NewOutbox(w, message.FakeValidator{RejectMessages: true}, queue, publisher,
			message.NullPolicy{}, provider, provider, newOutboxTestJournal(t))

		cid, _, err := ob.Send(context.Background(), sender, sender, types.NewAttoFILFromFIL(2), types.NewGasPrice(0), gas.NewGas(0), bcast, builtin.MethodSend, adt.Empty)
		assert.Errorf(t, err, "for testing")
		assert.False(t, cid.Defined())
	})

	t.Run("send message enqueues and calls Publish, but respects bcast flag for broadcasting", func(t *testing.T) {
		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		toAddr := vmaddr.NewForTestGetter()()
		queue := message.NewQueue()
		publisher := &message.MockPublisher{}
		provider := message.NewFakeProvider(t)

		head := provider.BuildOneOn(block.UndefTipSet, func(b *chain.BlockBuilder) {
			b.IncHeight(1000)
		})
		actr := actor.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0), cid.Undef)
		actr.CallSeqNum = 42
		provider.SetHeadAndActor(t, head.Key(), sender, actr)

		ob := message.NewOutbox(w, message.FakeValidator{}, queue, publisher, message.NullPolicy{}, provider, provider, newOutboxTestJournal(t))
		require.Empty(t, queue.List(sender))
		require.Nil(t, publisher.Message)

		testCases := []struct {
			bcast  bool
			nonce  uint64
			height int
		}{{true, actr.CallSeqNum, 1000}, {false, actr.CallSeqNum + 1, 1000}}

		for _, test := range testCases {
			_, pubDone, err := ob.Send(context.Background(), sender, toAddr, types.ZeroAttoFIL, types.NewGasPrice(0), gas.NewGas(0), test.bcast, builtin.MethodSend, adt.Empty)
			require.NoError(t, err)
			assert.Equal(t, uint64(test.height), queue.List(sender)[0].Stamp)
			require.NotNil(t, pubDone)
			pubErr := <-pubDone
			assert.NoError(t, pubErr)
			require.NotNil(t, publisher.Message)
			assert.Equal(t, test.nonce, publisher.Message.Message.CallSeqNum)
			assert.Equal(t, abi.ChainEpoch(test.height), publisher.Height)
			assert.Equal(t, test.bcast, publisher.Bcast)
		}

	})
	t.Run("send message avoids nonce race", func(t *testing.T) {
		ctx := context.Background()
		msgCount := 20      // number of messages to send
		sendConcurrent := 3 // number of of concurrent message sends

		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		toAddr := vmaddr.NewForTestGetter()()
		queue := message.NewQueue()
		publisher := &message.MockPublisher{}
		provider := message.NewFakeProvider(t)
		bcast := true

		head := provider.BuildOneOn(block.UndefTipSet, func(b *chain.BlockBuilder) {
			b.IncHeight(1000)
		})
		actr := actor.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0), cid.Undef)
		actr.CallSeqNum = 42
		provider.SetHeadAndActor(t, head.Key(), sender, actr)

		s := message.NewOutbox(w, message.FakeValidator{}, queue, publisher, message.NullPolicy{}, provider, provider, newOutboxTestJournal(t))

		var wg sync.WaitGroup
		addTwentyMessages := func(batch int) {
			defer wg.Done()
			for i := 0; i < msgCount; i++ {
				method := abi.MethodNum(batch*10000 + i)
				_, _, err := s.Send(ctx, sender, toAddr, types.ZeroAttoFIL, types.NewGasPrice(0), gas.NewGas(0), bcast, method, adt.Empty)
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
			_, found := nonces[message.Msg.Message.CallSeqNum]
			require.False(t, found)
			nonces[message.Msg.Message.CallSeqNum] = true
		}

		for i := 0; i < 60; i++ {
			assert.True(t, nonces[actr.CallSeqNum+uint64(i)])

		}
	})

	t.Run("fails with non-account actor", func(t *testing.T) {
		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		toAddr := vmaddr.NewForTestGetter()()
		queue := message.NewQueue()
		publisher := &message.MockPublisher{}
		provider := message.NewFakeProvider(t)

		head := provider.NewGenesis()
		actr := actor.NewActor(builtin.StorageMarketActorCodeID, abi.NewTokenAmount(0), cid.Undef)
		provider.SetHeadAndActor(t, head.Key(), sender, actr)

		ob := message.NewOutbox(w, message.FakeValidator{}, queue, publisher, message.NullPolicy{}, provider, provider, newOutboxTestJournal(t))

		_, _, err := ob.Send(context.Background(), sender, toAddr, types.ZeroAttoFIL, types.NewGasPrice(0), gas.NewGas(0), true, builtin.MethodSend, adt.Empty)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "account or empty")
	})
}
