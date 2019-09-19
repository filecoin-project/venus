package testhelpers_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestFakeClockTimers(t *testing.T) {
	tf.UnitTest(t)
	fc := th.NewFakeSystemClock(time.Now())

	zero := fc.NewTimer(0)

	assert.False(t, zero.Stop(), "zero timer could be stopped")
	select {
	case <-zero.Chan():
	default:
		t.Errorf("zero timer didn't emit time")
	}

	one := fc.NewTimer(1)

	select {
	case <-one.Chan():
		t.Errorf("non-zero timer did emit time")
	default:
	}

	assert.True(t, one.Stop(), "non-zero timer couldn't be stopped")

	fc.Advance(5)

	select {
	case <-one.Chan():
		t.Errorf("stopped timer did emit time")
	default:
	}

	assert.False(t, one.Reset(1), "resetting stopped timer didn't return false")
	assert.True(t, one.Reset(1), "resetting active timer didn't return true")

	fc.Advance(1)

	assert.False(t, one.Stop(), "triggered timer could be stopped")

	select {
	case <-one.Chan():
	default:
		t.Errorf("triggered timer didn't emit time")
	}

	fc.Advance(1)

	select {
	case <-one.Chan():
		t.Errorf("triggered timer emitted time more than once")
	default:
	}

	one.Reset(0)

	assert.False(t, one.Stop(), "reset to zero timer could be stopped")
	select {
	case <-one.Chan():
	default:
		t.Errorf("reset to zero timer didn't emit time")
	}
}
