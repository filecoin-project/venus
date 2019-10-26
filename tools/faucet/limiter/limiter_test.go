package limiter

import (
	"testing"
	"time"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

type MockTime struct {
	UntilReturn time.Duration
}

func (mt *MockTime) Until(t time.Time) time.Duration {
	return mt.UntilReturn
}

func TestReady(t *testing.T) {
	tf.UnitTest(t)

	addr := "Qmaddr"

	t.Run("Not ready before time elapses", func(t *testing.T) {
		lockedFor := time.Microsecond * 50
		mt := &MockTime{}
		mt.UntilReturn = lockedFor

		l := NewLimiter(mt)

		l.Add(addr, time.Now().Add(lockedFor))
		t0, ok := l.Ready(addr)
		assert.False(t, ok)
		assert.Equal(t, lockedFor, t0)
	})

	t.Run("Ready after time elapses", func(t *testing.T) {
		lockedFor := time.Microsecond * 50

		mt := &MockTime{}
		mt.UntilReturn = time.Duration(0)

		l := NewLimiter(mt)

		l.Add(addr, time.Now().Add(lockedFor))
		t0, ok := l.Ready(addr)
		assert.True(t, ok)
		assert.Equal(t, time.Duration(0), t0)
	})

	t.Run("Ready if not added", func(t *testing.T) {
		mt := &MockTime{}
		mt.UntilReturn = time.Duration(0)

		l := NewLimiter(mt)

		t0, ok := l.Ready(addr)
		assert.True(t, ok)
		assert.Equal(t, time.Duration(0), t0)
	})

	t.Run("Ready after waiting returned duration", func(t *testing.T) {
		lockedFor := time.Microsecond * 50
		mt := &MockTime{}
		mt.UntilReturn = lockedFor

		l := NewLimiter(mt)

		l.Add(addr, time.Now().Add(lockedFor))

		d0, ok := l.Ready(addr)
		assert.False(t, ok)
		assert.Equal(t, lockedFor, d0)

		mt.UntilReturn = time.Duration(0)

		d0, ok = l.Ready(addr)
		assert.True(t, ok)
		assert.Equal(t, time.Duration(0), d0)
	})
}

func TestClear(t *testing.T) {
	tf.UnitTest(t)

	addr := "Qmaddr"

	t.Run("Ready after clear", func(t *testing.T) {
		lockedFor := time.Microsecond * 50
		mt := &MockTime{}
		mt.UntilReturn = lockedFor

		l := NewLimiter(mt)

		l.Add(addr, time.Now().Add(lockedFor))
		_, ok := l.Ready(addr)
		assert.False(t, ok)

		l.Clear(addr)

		_, ok = l.Ready(addr)
		assert.True(t, ok)
	})
}

func TestClean(t *testing.T) {
	tf.UnitTest(t)

	addr := "Qmaddr"

	t.Run("Removes expired values", func(t *testing.T) {
		lockedFor := time.Microsecond * 50
		mt := &MockTime{}
		mt.UntilReturn = time.Duration(0)

		l := NewLimiter(mt)

		l.Add(addr, time.Now().Add(lockedFor))
		assert.Len(t, l.addrs, 1)

		l.Clean()

		assert.Len(t, l.addrs, 0)

		_, ok := l.Ready(addr)
		assert.True(t, ok)
	})
}
