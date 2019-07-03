package porcelain_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/porcelain"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

type fakeMpoolWaitPlumbing struct {
	pending            []*types.SignedMessage
	subscription       *pubsub.FakeSubscription // Receives subscription as is it opened
	afterPendingCalled func()                   // Invoked after each call to MessagePoolPending
}

func newFakeMpoolWaitPlumbing(onPendingCalled func()) *fakeMpoolWaitPlumbing {
	return &fakeMpoolWaitPlumbing{
		afterPendingCalled: onPendingCalled,
	}
}

func (plumbing *fakeMpoolWaitPlumbing) MessagePoolPending() []*types.SignedMessage {
	if plumbing.afterPendingCalled != nil {
		defer plumbing.afterPendingCalled()
	}
	return plumbing.pending
}

func (plumbing *fakeMpoolWaitPlumbing) PubSubSubscribe(topic string) (pubsub.Subscription, error) {
	subscription := pubsub.NewFakeSubscription(net.MessageTopic, 1)
	plumbing.subscription = subscription
	return subscription, nil
}

func TestMessagePoolWait(t *testing.T) {
	tf.UnitTest(t)

	ki := types.MustGenerateKeyInfo(1, 42)
	signer := types.NewMockSigner(ki)

	t.Run("empty", func(t *testing.T) {

		plumbing := newFakeMpoolWaitPlumbing(nil)
		msgs, e := porcelain.MessagePoolWait(context.Background(), plumbing, 0)
		require.NoError(t, e)
		assert.Equal(t, 0, len(msgs))
	})

	t.Run("returns immediates", func(t *testing.T) {

		plumbing := newFakeMpoolWaitPlumbing(nil)
		plumbing.pending = types.NewSignedMsgs(3, signer)

		msgs, e := porcelain.MessagePoolWait(context.Background(), plumbing, 3)
		require.NoError(t, e)
		assert.Equal(t, 3, len(msgs))
	})

	t.Run("waits", func(t *testing.T) {

		var plumbing *fakeMpoolWaitPlumbing
		callCount := 0

		// This callback to the MessagePoolPending plumbing orchestrates the appearance of
		// pending messages and notifications on the pubsub subscription.
		handlePendingCalled := func() {
			if callCount == 0 {
				// The first call is checking for the fast path; do nothing.
			} else if callCount == 1 {
				// Pubsub subscribed but not yet waited.
				// Bump the pubsub but *don't* add to message pool; the waiter must wait longer.
				plumbing.subscription.Post(nil)
			} else if callCount == 2 {
				// First pubsub bump processed.
				// Add a message to the pool then bump pubsub again.
				plumbing.pending = types.NewSignedMsgs(1, signer)
				plumbing.subscription.Post(nil)
			}
			callCount++
		}

		plumbing = newFakeMpoolWaitPlumbing(handlePendingCalled)
		finished := assertMessagePoolWaitAsync(plumbing, 1, t)

		finished.Wait()
		plumbing.subscription.AwaitCancellation()
	})

	t.Run("message races pubsub", func(t *testing.T) {

		var plumbing *fakeMpoolWaitPlumbing

		handlePendingCalled := func() {
			// The first call is checking for the fast path. It returns empty, but
			// then a message appears, racing the pubsub subscription.
			plumbing.pending = types.NewSignedMsgs(1, signer)
		}

		plumbing = newFakeMpoolWaitPlumbing(handlePendingCalled)
		finished := assertMessagePoolWaitAsync(plumbing, 1, t)

		finished.Wait()
		plumbing.subscription.AwaitCancellation()
	})
}

// assertMessagePoolWaitAsync waits for msgCount messages asynchronously
func assertMessagePoolWaitAsync(plumbing *fakeMpoolWaitPlumbing, msgCount uint, t *testing.T) *sync.WaitGroup {
	finished := sync.WaitGroup{}
	finished.Add(1)

	go func() {
		msgs, e := porcelain.MessagePoolWait(context.Background(), plumbing, msgCount)
		require.NoError(t, e)
		assert.Equal(t, msgCount, uint(len(msgs)))
		defer finished.Done()
	}()

	return &finished
}
