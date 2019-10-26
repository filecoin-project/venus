package moresync_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/moresync"
)

func TestLatch(t *testing.T) {
	tf.UnitTest(t)

	t.Run("zero count is open", func(t *testing.T) {
		l := moresync.NewLatch(0)
		assert.Equal(t, uint(0), l.Count())
		l.Wait() // Shouldn't block
	})

	t.Run("wait blocks until done", func(t *testing.T) {
		l := moresync.NewLatch(1)
		assert.Equal(t, uint(1), l.Count())
		waitDone := make(chan struct{})

		go func() {
			l.Wait() // Should block at first.
			close(waitDone)
		}()

		select {
		case <-waitDone:
			assert.Fail(t, "wait didn't")
		case <-time.After(time.Millisecond):
		}
		assert.Equal(t, uint(1), l.Count())

		l.Done()
		<-waitDone
		assert.Equal(t, uint(0), l.Count())
	})

	t.Run("counts down", func(t *testing.T) {
		l := moresync.NewLatch(5)
		for i := uint(5); i > 0; i-- {
			assert.Equal(t, i, l.Count())
			l.Done()
		}
		assert.Equal(t, uint(0), l.Count())
		l.Wait() // Shouldn't block
	})
}
