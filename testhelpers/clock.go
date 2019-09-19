package testhelpers

import (
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/filecoin-project/go-filecoin/clock"
)

// FakeSystemClock wrapps the jonboulle/clockwork.FakeClock interface.
// FakeSystemClock provides an interface for a clock which can be
// manually advanced through time.
type FakeSystemClock interface {
	clockwork.FakeClock
	NewTimer(d time.Duration) clock.Timer
	AfterFunc(d time.Duration, f func()) clock.Timer
}

type fakeSystemClock struct {
	clockwork.FakeClock
	timersLk sync.RWMutex
	timers   []*fakeTimer
}

// NewFakeSystemClock returns a FakeSystemClock initialised at the given time.Time.
func NewFakeSystemClock(n time.Time) FakeSystemClock {
	return &fakeSystemClock{FakeClock: clockwork.NewFakeClockAt(n)}
}

func (fsc *fakeSystemClock) NewTimer(d time.Duration) clock.Timer {
	out := make(chan time.Time, 1)
	sendTime := func(c interface{}, now time.Time) {
		c.(chan time.Time) <- now
	}

	ft := &fakeTimer{
		clock:    fsc,
		until:    fsc.Now().Add(d),
		callback: sendTime,
		arg:      out,
		out:      out,
	}
	fsc.addTimer(ft)
	return ft
}

func (fsc *fakeSystemClock) AfterFunc(d time.Duration, f func()) clock.Timer {
	goFunc := func(fn interface{}, _ time.Time) {
		go fn.(func())()
	}

	s := &fakeTimer{
		clock:    fsc,
		until:    fsc.Now().Add(d),
		callback: goFunc,
		arg:      f,
		// zero-valued c, the same as it is in the `time` pkg
	}
	fsc.addTimer(s)
	return s
}

func (fsc *fakeSystemClock) addTimer(ft *fakeTimer) {

	now := fsc.Now()
	if now.Sub(ft.until) >= 0 {
		ft.awaken(now)
	} else {
		fsc.timersLk.Lock()
		fsc.timers = append(fsc.timers, ft)
		fsc.timersLk.Unlock()
	}
}

func (fsc *fakeSystemClock) Advance(d time.Duration) {
	fsc.FakeClock.Advance(d)
	now := fsc.Now()
	fsc.timersLk.Lock()
	var newTimers []*fakeTimer
	for _, ft := range fsc.timers {
		if now.Sub(ft.until) >= 0 {
			ft.awaken(now)
		} else {
			newTimers = append(newTimers, ft)
		}
	}
	fsc.timers = newTimers
	fsc.timersLk.Unlock()
}

type fakeTimer struct {
	clock    *fakeSystemClock
	until    time.Time
	callback func(interface{}, time.Time)
	arg      interface{}
	doneLk   sync.RWMutex
	done     bool
	out      chan time.Time
}

func (ft *fakeTimer) awaken(now time.Time) {
	ft.doneLk.Lock()
	if !ft.done {
		ft.done = true
		ft.callback(ft.arg, now)
	}
	ft.doneLk.Unlock()
}

func (ft *fakeTimer) Chan() <-chan time.Time {
	return ft.out
}

func (ft *fakeTimer) Reset(d time.Duration) bool {
	active := ft.Stop()
	ft.until = ft.clock.Now().Add(d)
	ft.doneLk.Lock()
	ft.done = false
	ft.doneLk.Unlock()
	ft.clock.addTimer(ft)
	return active
}

func (ft *fakeTimer) Stop() bool {
	ft.doneLk.Lock()
	wasActive := !ft.done
	ft.done = true
	ft.doneLk.Unlock()

	if wasActive {
		ft.until = ft.clock.Now()
		ft.clock.Advance(0)
	}
	return wasActive
}
