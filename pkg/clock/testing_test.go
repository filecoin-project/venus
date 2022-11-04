// stm: #unit
package clock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/pkg/clock"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

var startTime = time.Unix(123456789, 0)

func TestFakeAfter(t *testing.T) {
	tf.UnitTest(t)
	fc := clock.NewFake(startTime)

	zero := fc.After(0)
	select {
	case <-zero:
	default:
		t.Errorf("zero did not return!")
	}
	// stm: @CLOCK_TESTING_AFTER_001
	one := fc.After(1)
	two := fc.After(2)
	six := fc.After(6)
	ten := fc.After(10)
	fc.Advance(1)
	select {
	case <-one:
	default:
		t.Errorf("one did not return!")
	}
	select {
	case <-two:
		t.Errorf("two returned prematurely!")
	case <-six:
		t.Errorf("six returned prematurely!")
	case <-ten:
		t.Errorf("ten returned prematurely!")
	default:
	}
	fc.Advance(1)
	select {
	case <-two:
	default:
		t.Errorf("two did not return!")
	}
	select {
	case <-six:
		t.Errorf("six returned prematurely!")
	case <-ten:
		t.Errorf("ten returned prematurely!")
	default:
	}
	fc.Advance(1)
	select {
	case <-six:
		t.Errorf("six returned prematurely!")
	case <-ten:
		t.Errorf("ten returned prematurely!")
	default:
	}
	fc.Advance(3)
	select {
	case <-six:
	default:
		t.Errorf("six did not return!")
	}
	select {
	case <-ten:
		t.Errorf("ten returned prematurely!")
	default:
	}
	fc.Advance(100)
	select {
	case <-ten:
	default:
		t.Errorf("ten did not return!")
	}
}

func TestNewFakeAt(t *testing.T) {
	tf.UnitTest(t)
	t1 := time.Date(1999, time.February, 3, 4, 5, 6, 7, time.UTC)
	fc := clock.NewFake(t1)
	// stm: @CLOCK_TESTING_NOW_001
	now := fc.Now()
	assert.Equalf(t, now, t1, "Fake.Now() returned unexpected non-initialised value: want=%#v, got %#v", t1, now)
}

func TestFakeSince(t *testing.T) {
	tf.UnitTest(t)
	fc := clock.NewFake(startTime)
	now := fc.Now()
	elapsedTime := time.Second
	fc.Advance(elapsedTime)
	// stm: @CLOCK_TESTING_SINCE_001
	assert.Truef(t, fc.Since(now) == elapsedTime, "Fake.Since() returned unexpected duration, got: %d, want: %d", fc.Since(now), elapsedTime)
}

func TestFakeTimers(t *testing.T) {
	tf.UnitTest(t)
	fc := clock.NewFake(startTime)

	// stm: @CLOCK_TESTING_NEW_TIMER_001
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

	// stm: @CLOCK_TESTING_ADVANCE_001
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

type syncFunc func(didAdvance func(), shouldAdvance func(string), shouldBlock func(string))

func inSync(t *testing.T, func1 syncFunc, func2 syncFunc) {
	stepChan1 := make(chan struct{}, 16)
	stepChan2 := make(chan struct{}, 16)
	go func() {
		func1(
			func() {
				stepChan1 <- struct{}{}
			},
			func(point string) {
				select {
				case <-stepChan2:
				case <-time.After(time.Second):
					t.Errorf("Did not advance, should have %s", point)
				}
			},
			func(point string) {
				select {
				case <-stepChan2:
					t.Errorf("Was able to advance, should not have %s", point)
				case <-time.After(10 * time.Millisecond):
				}
			},
		)
	}()
	func2(func() { stepChan2 <- struct{}{} }, func(point string) {
		select {
		case <-stepChan1:
		case <-time.After(time.Second):
			t.Errorf("Did not advance, should have %s", point)
		}
	},
		func(point string) {
			select {
			case <-stepChan1:
				t.Errorf("Was able to advance, should not have %s", point)
			case <-time.After(10 * time.Millisecond):
			}
		})
}

func TestBlockingOnTimers(t *testing.T) {
	tf.UnitTest(t)
	fc := clock.NewFake(startTime)

	inSync(t, func(didAdvance func(), shouldAdvance func(string), _ func(string)) {
		// stm: @CLOCK_TESTING_BLOCK_UNTIL_001
		fc.BlockUntil(0)
		didAdvance()
		fc.BlockUntil(1)
		didAdvance()
		shouldAdvance("timers stopped")
		fc.BlockUntil(0)
		didAdvance()
		fc.BlockUntil(1)
		didAdvance()
		fc.BlockUntil(2)
		didAdvance()
		fc.BlockUntil(3)
		didAdvance()
		shouldAdvance("timers stopped")
		fc.BlockUntil(2)
		didAdvance()
		shouldAdvance("time advanced")
		fc.BlockUntil(0)
		didAdvance()
	}, func(didAdvance func(), shouldAdvance func(string), shouldBlock func(string)) {
		shouldAdvance("when only blocking for 0 timers")
		shouldBlock("when waiting for 1 timer")
		fc.NewTimer(0)
		shouldBlock("when immediately expired timer added")
		one := fc.NewTimer(1)
		shouldAdvance("once a timer exists")
		one.Stop()
		didAdvance()
		shouldAdvance("when only blocking for 0 timers")
		shouldBlock("when all timers are stopped and waiting for a timer")
		one.Reset(1)
		shouldAdvance("once timer is restarted")
		shouldBlock("when waiting for 2 timers with one active")
		_ = fc.NewTimer(2)
		shouldAdvance("when second timer added")
		shouldBlock("when waiting for 3 timers with 2 active")
		_ = fc.NewTimer(3)
		shouldAdvance("when third timer added")
		one.Stop()
		didAdvance()
		shouldAdvance("when blocking for 2 timers if a third is stopped")
		fc.Advance(3)
		didAdvance()
		shouldAdvance("waiting for no timers")
	})
}

func TestAdvancePastAfter(t *testing.T) {
	tf.UnitTest(t)

	fc := clock.NewFake(startTime)

	start := fc.Now()
	// stm: @CLOCK_TESTING_AFTER_FUNC_001
	one := fc.After(1)
	two := fc.After(2)
	six := fc.After(6)

	fc.Advance(1)
	assert.False(t, start.Add(1).Sub(<-one) > 0, "timestamp is too early")

	fc.Advance(5)
	assert.False(t, start.Add(2).Sub(<-two) > 0, "timestamp is too early")
	assert.False(t, start.Add(6).Sub(<-six) > 0, "timestamp is too early")
}

func TestFakeTickerStop(t *testing.T) {
	tf.UnitTest(t)
	fc := clock.NewFake(startTime)

	ft := fc.NewTicker(1)
	ft.Stop()
	fc.Advance(1)
	select {
	case <-ft.Chan():
		t.Errorf("received unexpected tick!")
	default:
	}
}

func TestFakeTickerTick(t *testing.T) {
	tf.UnitTest(t)
	fc := clock.NewFake(startTime)
	now := fc.Now()

	// The tick at now.Add(2) should not get through since we advance time by
	// two units below and the channel can hold at most one tick until it's
	// consumed.
	first := now.Add(1)
	second := now.Add(3)

	// We wrap the Advance() calls with blockers to make sure that the ticker
	// can go to sleep and produce ticks without time passing in parallel.
	ft := fc.NewTicker(1)
	fc.BlockUntil(1)
	fc.Advance(2)
	fc.BlockUntil(1)

	select {
	case tick := <-ft.Chan():
		assert.Truef(t, tick == first, "wrong tick time, got: %v, want: %v", tick, first)
	default:
		t.Errorf("expected tick!")
	}

	// Advance by one more unit, we should get another tick now.
	fc.Advance(1)
	fc.BlockUntil(1)

	select {
	case tick := <-ft.Chan():
		assert.Truef(t, tick == second, "wrong tick time, got: %v, want: %v", tick, second)
	default:
		t.Errorf("expected tick!")
	}
	ft.Stop()
}

func TestFakeSleep(t *testing.T) {
	tf.UnitTest(t)
	fc := clock.NewFake(startTime)
	var afterSleep = make(chan struct{})

	go func() {
		go func() {
			<-time.After(time.Second)
			fc.Advance(1)
		}()
		// stm: @CLOCK_TESTING_SLEEP_001
		fc.Sleep(1)
		afterSleep <- struct{}{}
	}()

	select {
	case <-time.After(time.Second * 5):
		t.Fatalf("wakeup sleep timeout.")
	case <-afterSleep:
		return
	}
}
