package limiter

import (
	"testing"
	"time"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

type MockTime struct {
	UntilReturn time.Duration
}

func (mt *MockTime) Until(t time.Time) time.Duration {
	return mt.UntilReturn
}

func TestReady(t *testing.T) {
	addr := "Qmaddr"

	t.Run("Not ready before time elapses", func(t *testing.T) {
		assert := assert.New(t)

		lockedFor := time.Microsecond * 50
		mt := &MockTime{}
		mt.UntilReturn = lockedFor

		l := NewLimiter(mt)

		l.Add(addr, time.Now().Add(lockedFor))
		t0, ok := l.Ready(addr)
		assert.False(ok)
		assert.Equal(lockedFor, t0)
	})

	t.Run("Ready after time elapses", func(t *testing.T) {
		assert := assert.New(t)

		lockedFor := time.Microsecond * 50

		mt := &MockTime{}
		mt.UntilReturn = time.Duration(0)

		l := NewLimiter(mt)

		l.Add(addr, time.Now().Add(lockedFor))
		t0, ok := l.Ready(addr)
		assert.True(ok)
		assert.Equal(time.Duration(0), t0)
	})

	t.Run("Ready if not added", func(t *testing.T) {
		assert := assert.New(t)

		mt := &MockTime{}
		mt.UntilReturn = time.Duration(0)

		l := NewLimiter(mt)

		t0, ok := l.Ready(addr)
		assert.True(ok)
		assert.Equal(time.Duration(0), t0)
	})

	t.Run("Ready after waiting returned duration", func(t *testing.T) {
		assert := assert.New(t)

		lockedFor := time.Microsecond * 50
		mt := &MockTime{}
		mt.UntilReturn = lockedFor

		l := NewLimiter(mt)

		l.Add(addr, time.Now().Add(lockedFor))

		d0, ok := l.Ready(addr)
		assert.False(ok)
		assert.Equal(lockedFor, d0)

		mt.UntilReturn = time.Duration(0)

		d0, ok = l.Ready(addr)
		assert.True(ok)
		assert.Equal(time.Duration(0), d0)
	})
}

func TestClear(t *testing.T) {
	addr := "Qmaddr"

	t.Run("Ready after clear", func(t *testing.T) {
		assert := assert.New(t)

		lockedFor := time.Microsecond * 50
		mt := &MockTime{}
		mt.UntilReturn = lockedFor

		l := NewLimiter(mt)

		l.Add(addr, time.Now().Add(lockedFor))
		_, ok := l.Ready(addr)
		assert.False(ok)

		l.Clear(addr)

		_, ok = l.Ready(addr)
		assert.True(ok)
	})
}

func TestClean(t *testing.T) {
	addr := "Qmaddr"

	t.Run("Removes expired values", func(t *testing.T) {
		assert := assert.New(t)

		lockedFor := time.Microsecond * 50
		mt := &MockTime{}
		mt.UntilReturn = time.Duration(0)

		l := NewLimiter(mt)

		l.Add(addr, time.Now().Add(lockedFor))
		assert.Len(l.addrs, 1)

		l.Clean()

		assert.Len(l.addrs, 0)

		_, ok := l.Ready(addr)
		assert.True(ok)
	})
}
