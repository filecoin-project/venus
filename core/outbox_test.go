package core_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutbox(t *testing.T) {
	tf.UnitTest(t)

	t.Run("invalid message rejected", func(t *testing.T) {
		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		queue := core.NewMessageQueue()
		publisher := &mockPublisher{}
		provider := &fakeProvider{}

		ob := core.NewOutbox(w, nullValidator{rejectMessages: true}, queue, publisher, provider, provider)

		cid, err := ob.Send(context.Background(), sender, sender, types.NewAttoFILFromFIL(2), types.NewGasPrice(0), types.NewGasUnits(0), "")
		assert.Errorf(t, err, "for testing")
		assert.False(t, cid.Defined())
	})

	t.Run("send message enqueues and calls publish", func(t *testing.T) {
		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		toAddr := address.NewForTestGetter()()
		queue := core.NewMessageQueue()
		publisher := &mockPublisher{}
		provider := &fakeProvider{}

		blk := types.NewBlockForTest(nil, 1)
		blk.Height = 1000
		actr, _ := account.NewActor(types.NewZeroAttoFIL())
		actr.Nonce = 42
		provider.Set(t, blk, sender, actr)

		ob := core.NewOutbox(w, nullValidator{}, queue, publisher, provider, provider)
		require.Empty(t, queue.List(sender))
		require.Nil(t, publisher.message)

		_, err := ob.Send(context.Background(), sender, toAddr, types.NewZeroAttoFIL(), types.NewGasPrice(0), types.NewGasUnits(0), "")
		require.NoError(t, err)
		assert.Equal(t, uint64(1000), queue.List(sender)[0].Stamp)
		assert.NotNil(t, publisher.message)
		assert.Equal(t, actr.Nonce, publisher.message.Nonce)
		assert.Equal(t, uint64(1000), publisher.height)
	})

	t.Run("send message avoids nonce race", func(t *testing.T) {
		ctx := context.Background()
		msgCount := 20      // number of messages to send
		sendConcurrent := 3 // number of of concurrent message sends

		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		toAddr := address.NewForTestGetter()()
		queue := core.NewMessageQueue()
		publisher := &mockPublisher{}
		provider := &fakeProvider{}

		blk := types.NewBlockForTest(nil, 1)
		blk.Height = 1000
		actr, _ := account.NewActor(types.NewZeroAttoFIL())
		actr.Nonce = 42
		provider.Set(t, blk, sender, actr)

		s := core.NewOutbox(w, nullValidator{}, queue, publisher, provider, provider)

		var wg sync.WaitGroup
		addTwentyMessages := func(batch int) {
			defer wg.Done()
			for i := 0; i < msgCount; i++ {
				_, err := s.Send(ctx, sender, toAddr, types.NewZeroAttoFIL(), types.NewGasPrice(0), types.NewGasUnits(0), fmt.Sprintf("%d-%d", batch, i), []byte{})
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

	t.Run("nonce fails with non-account actor", func(t *testing.T) {
		w, _ := types.NewMockSignersAndKeyInfo(1)
		sender := w.Addresses[0]
		toAddr := address.NewForTestGetter()()
		queue := core.NewMessageQueue()
		publisher := &mockPublisher{}
		provider := &fakeProvider{}

		blk := types.NewBlockForTest(nil, 1)
		actr, _ := storagemarket.NewActor() // Not an account actor
		provider.Set(t, blk, sender, actr)

		ob := core.NewOutbox(w, nullValidator{}, queue, publisher, provider, provider)

		_, err := ob.Send(context.Background(), sender, toAddr, types.NewZeroAttoFIL(), types.NewGasPrice(0), types.NewGasUnits(0), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "account or empty")
	})
}

// A publisher which just stores the last message published.
type mockPublisher struct {
	returnError error                // Error to be returned by Publish()
	message     *types.SignedMessage // Message received by Publish()
	height      uint64               // Height received by Publish()
}

func (p *mockPublisher) Publish(ctx context.Context, message *types.SignedMessage, height uint64) error {
	p.message = message
	p.height = height
	return p.returnError
}

// A chain and actor provider which provides and expects values for a single message.
type fakeProvider struct {
	head   types.SortedCidSet // Provided by GetHead and expected by others
	tipset *types.TipSet      // Provided by GetTipset(head)
	addr   address.Address    // Expected by GetActor
	actor  *actor.Actor       // Provided by GetActor(head, addr)
}

func (p *fakeProvider) GetHead() types.SortedCidSet {
	return p.head
}

func (p *fakeProvider) GetTipSet(tsKey types.SortedCidSet) (*types.TipSet, error) {
	if !tsKey.Equals(p.head) {
		return nil, errors.Errorf("No such tipset %s, expected %s", tsKey, p.head)
	}
	return p.tipset, nil
}

func (p *fakeProvider) GetActor(ctx context.Context, tsKey types.SortedCidSet, addr address.Address) (*actor.Actor, error) {
	if !tsKey.Equals(p.head) {
		return nil, errors.Errorf("No such tipset %s, expected %s", tsKey, p.head)
	}
	if addr != p.addr {
		return nil, errors.Errorf("No such address %s, expected %s", addr, p.addr)
	}
	return p.actor, nil
}

// Set sets the tipset, from address, and actor to be provided by creating a tipset with one block.
func (p *fakeProvider) Set(t *testing.T, block *types.Block, addr address.Address, actor *actor.Actor) {
	ts := types.RequireNewTipSet(t, block)
	p.tipset = &ts
	tsKey := types.NewSortedCidSet(block.Cid())
	p.head = tsKey
	p.addr = addr
	p.actor = actor
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
