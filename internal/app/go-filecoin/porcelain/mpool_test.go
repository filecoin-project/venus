package porcelain_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

type fakeMpoolWaitPlumbing struct {
	pending            []*types.SignedMessage
	afterPendingCalled func() // Invoked after each call to MessagePoolPending
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
			} else if callCount == 2 {
				// Add a message to the pool.
				plumbing.pending = types.NewSignedMsgs(1, signer)
			}
			callCount++
		}

		plumbing = newFakeMpoolWaitPlumbing(handlePendingCalled)
		finished := assertMessagePoolWaitAsync(plumbing, 1, t)

		finished.Wait()
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
