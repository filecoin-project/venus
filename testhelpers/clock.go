package testhelpers

import (
	"sync"
	"time"

	"github.com/filecoin-project/go-filecoin/clock"
)

// FakeSystemClock provides an interface for a clock which can be
// manually advanced through time
// Adapted from: https://github.com/jonboulle/clockwork
type FakeSystemClock interface {
	clock.Clock
	// Advance advances the FakeClock to a new point in time, ensuring any existing
	// sleepers are notified appropriately before returning
	Advance(d time.Duration)
	// BlockUntil will block until the FakeClock has the given number of
	// sleepers (callers of Sleep or After)
	BlockUntil(n int)
}

// NewFakeSystemClock returns a FakeSystemClock initialised at the given time.Time.
func NewFakeSystemClock(n time.Time) FakeSystemClock {
	return &fakeClock{
		time: n,
	}
}

type fakeClock struct {
	timers   []*timer
	blockers []*blocker
	time     time.Time

	l sync.RWMutex
}

// timer represents a waiting timer from NewTimer, Sleep, After, etc.
type timer struct {
	until    time.Time
	callback func(interface{}, time.Time)
	arg      interface{}

	c      chan time.Time
	doneLk sync.RWMutex
	done   bool
	clock  *fakeClock // needed for Reset()
}

// blocker represents a caller of BlockUntil
type blocker struct {
	count int
	ch    chan struct{}
}

func (s *timer) awaken(now time.Time) {
	s.doneLk.Lock()
	defer s.doneLk.Unlock()
	if !s.done {
		s.done = true
		s.callback(s.arg, now)
	}
}

func (s *timer) Chan() <-chan time.Time { return s.c }

func (s *timer) Reset(d time.Duration) bool {
	wasActive := s.Stop()
	s.until = s.clock.Now().Add(d)
	s.doneLk.Lock()
	s.done = false
	s.doneLk.Unlock()
	s.clock.addTimer(s)
	return wasActive
}

func (s *timer) Stop() bool {
	s.doneLk.Lock()
	if s.done {
		s.doneLk.Unlock()
		return false
	}
	s.done = true
	s.doneLk.Unlock()
	// Expire the timer and notify blockers
	s.until = s.clock.Now()
	s.clock.Advance(0)
	return true
}

func (fc *fakeClock) addTimer(s *timer) {
	fc.l.Lock()
	defer fc.l.Unlock()

	now := fc.time
	if now.Sub(s.until) >= 0 {
		// special case - trigger immediately
		s.awaken(now)
	} else {
		// otherwise, add to the set of sleepers
		fc.timers = append(fc.timers, s)
		// and notify any blockers
		fc.blockers = notifyBlockers(fc.blockers, len(fc.timers))
	}
}

// After mimics time.After; it waits for the given duration to elapse on the
// fakeClock, then sends the current time on the returned channel.
func (fc *fakeClock) After(d time.Duration) <-chan time.Time {
	return fc.NewTimer(d).Chan()
}

// notifyBlockers notifies all the blockers waiting until the
// given number of sleepers are waiting on the fakeClock. It
// returns an updated slice of blockers (i.e. those still waiting)
func notifyBlockers(blockers []*blocker, count int) (newBlockers []*blocker) {
	for _, b := range blockers {
		if b.count == count {
			close(b.ch)
		} else {
			newBlockers = append(newBlockers, b)
		}
	}
	return
}

// Sleep blocks until the given duration has passed on the fakeClock
func (fc *fakeClock) Sleep(d time.Duration) {
	<-fc.After(d)
}

// Time returns the current time of the fakeClock
func (fc *fakeClock) Now() time.Time {
	fc.l.RLock()
	t := fc.time
	fc.l.RUnlock()
	return t
}

// Since returns the duration that has passed since the given time on the fakeClock
func (fc *fakeClock) Since(t time.Time) time.Duration {
	return fc.Now().Sub(t)
}

func (fc *fakeClock) NewTicker(d time.Duration) clock.Ticker {
	ft := &fakeTicker{
		c:      make(chan time.Time, 1),
		stop:   make(chan bool, 1),
		clock:  fc,
		period: d,
	}
	go ft.tick()
	return ft
}

// NewTimer creates a new Timer that will send the current time on its channel
// after the given duration elapses on the fake clock.
func (fc *fakeClock) NewTimer(d time.Duration) clock.Timer {
	done := make(chan time.Time, 1)
	sendTime := func(c interface{}, now time.Time) {
		c.(chan time.Time) <- now
	}

	s := &timer{
		clock:    fc,
		until:    fc.time.Add(d),
		callback: sendTime,
		arg:      done,
		c:        done,
	}
	fc.addTimer(s)
	return s
}

// AfterFunc waits for the duration to elapse on the fake clock and then calls f
// in its own goroutine.
// It returns a Timer that can be used to cancel the call using its Stop method.
func (fc *fakeClock) AfterFunc(d time.Duration, f func()) clock.Timer {
	goFunc := func(fn interface{}, _ time.Time) {
		go fn.(func())()
	}

	s := &timer{
		clock:    fc,
		until:    fc.time.Add(d),
		callback: goFunc,
		arg:      f,
		// zero-valued c, the same as it is in the `time` pkg
	}
	fc.addTimer(s)
	return s
}

// Advance advances fakeClock to a new point in time, ensuring channels from any
// previous invocations of After are notified appropriately before returning
func (fc *fakeClock) Advance(d time.Duration) {
	fc.l.Lock()
	defer fc.l.Unlock()

	end := fc.time.Add(d)
	var newSleepers []*timer
	for _, s := range fc.timers {
		if end.Sub(s.until) >= 0 {
			s.awaken(end)
		} else {
			newSleepers = append(newSleepers, s)
		}
	}
	fc.timers = newSleepers
	fc.blockers = notifyBlockers(fc.blockers, len(fc.timers))
	fc.time = end
}

// BlockUntil will block until the fakeClock has the given number of sleepers
// (callers of Sleep or After)
func (fc *fakeClock) BlockUntil(n int) {
	fc.l.Lock()
	// Fast path: current number of sleepers is what we're looking for
	if len(fc.timers) == n {
		fc.l.Unlock()
		return
	}
	// Otherwise, set up a new blocker
	b := &blocker{
		count: n,
		ch:    make(chan struct{}),
	}
	fc.blockers = append(fc.blockers, b)
	fc.l.Unlock()
	<-b.ch
}

type fakeTicker struct {
	c      chan time.Time
	stop   chan bool
	clock  FakeSystemClock
	period time.Duration
}

func (ft *fakeTicker) Chan() <-chan time.Time {
	return ft.c
}

func (ft *fakeTicker) Stop() {
	ft.stop <- true
}

// tick sends the tick time to the ticker channel after every period.
// Tick events are discarded if the underlying ticker channel does
// not have enough capacity.
func (ft *fakeTicker) tick() {
	tick := ft.clock.Now()
	for {
		tick = tick.Add(ft.period)
		remaining := tick.Sub(ft.clock.Now())
		if remaining <= 0 {
			// The tick should have already happened. This can happen when
			// Advance() is called on the fake clock with a duration larger
			// than this ticker's period.
			select {
			case ft.c <- tick:
			default:
			}
			continue
		}

		select {
		case <-ft.stop:
			return
		case <-ft.clock.After(remaining):
			select {
			case ft.c <- tick:
			default:
			}
		}
	}
}
