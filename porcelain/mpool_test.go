package porcelain_test

import (
	"context"
	"sync"
	"testing"

	"gx/ipfs/QmepvmmYNM6q4RaUiwEikQFhgMFHXg2PLhx2E9iaRd3jmS/go-libp2p-pubsub"

	"github.com/filecoin-project/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/plumbing/ps"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMpoolWaitPlumbing struct {
	pending []*types.SignedMessage
	subCh   chan *ps.FakeSubscription // Receives subscription as is it opened
}

func newFakeMpoolWaitPlumbing() *fakeMpoolWaitPlumbing {
	return &fakeMpoolWaitPlumbing{
		subCh: make(chan *ps.FakeSubscription, 1),
	}
}

func (plumbing *fakeMpoolWaitPlumbing) MessagePoolPending() []*types.SignedMessage {
	return plumbing.pending
}

func (plumbing *fakeMpoolWaitPlumbing) PubSubSubscribe(topic string, opts ...pubsub.SubOpt) (ps.Subscription, error) {
	subscription := ps.NewFakeSubscription(msg.Topic)
	plumbing.subCh <- subscription
	close(plumbing.subCh)
	return subscription, nil
}

func TestMessagePoolWait(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)
	ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
	signer := types.NewMockSigner(ki)

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		plumbing := newFakeMpoolWaitPlumbing()
		msgs, e := porcelain.MessagePoolWait(context.Background(), plumbing, 0)
		require.NoError(e)
		assert.Equal(0, len(msgs))
	})

	t.Run("returns immediates", func(t *testing.T) {
		t.Parallel()
		plumbing := newFakeMpoolWaitPlumbing()
		plumbing.pending = types.NewSignedMsgs(3, signer)

		msgs, e := porcelain.MessagePoolWait(context.Background(), plumbing, 3)
		require.NoError(e)
		assert.Equal(3, len(msgs))
	})

	t.Run("waits", func(t *testing.T) {
		t.Parallel()
		plumbing := newFakeMpoolWaitPlumbing()

		wg := sync.WaitGroup{}
		wg.Add(2)

		// Waits for subscription to be requested, then posts a message
		go func() {
			sub := <-plumbing.subCh
			// First pubsub message doesn't affect the message pool.
			// The waiter must wait longer.
			sub.Post(nil)

			// Add a message to the pool then bump the pubsub again.
			plumbing.pending = types.NewSignedMsgs(1, signer)
			sub.Post(nil)

			sub.AwaitCancellation()
			defer wg.Done()
		}()

		go func() {
			msgs, e := porcelain.MessagePoolWait(context.Background(), plumbing, 1)
			require.NoError(e)
			assert.Equal(1, len(msgs))
			defer wg.Done()
		}()

		wg.Wait()
	})
}
