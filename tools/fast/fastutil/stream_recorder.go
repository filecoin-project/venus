package fastutil

import (
	"bytes"
	"sync"
)

// IntervalRecorder is an io.Writer which provides an easy way to record intervals of
// data written to it.
type IntervalRecorder struct {
	// buf is the internal buffer that stores all data written to the
	// IntervalRecorder until an interval is started, or stopped.
	bufMu sync.Mutex
	buf   bytes.Buffer

	intervalsMu sync.Mutex
	intervals   []*Interval
}

// Interval is a bytes.Buffer containing all data that has been written
// to an IntervalRecorder from the Intervals creation until Stop is called.
// You should not create this structure directly but instead use the Start
// method on the IntervalRecorder.
type Interval struct {
	bytes.Buffer
	done func()
}

// NewIntervalRecorder returns a new IntervalRecorder.
func NewIntervalRecorder() *IntervalRecorder {
	return &IntervalRecorder{}
}

// Write appends the contents of p to the IntervalRecorder. The return value is the
// length of p; err is always nil. If the internal buffer comes to large, Write will
// panic with bytes.ErrToLarge.
// See bytes.Buffer for more info: https://golang.org/pkg/bytes/#Buffer.Write
func (lw *IntervalRecorder) Write(p []byte) (int, error) {
	lw.bufMu.Lock()
	defer lw.bufMu.Unlock()
	return lw.buf.Write(p)
}

// Transfers all the data in the IntervalRecorder to each interval currently
// being tracked.
func (lw *IntervalRecorder) drain() {
	lw.bufMu.Lock()
	defer lw.bufMu.Unlock()
	buf := lw.buf.Bytes()

	lw.intervalsMu.Lock()
	defer lw.intervalsMu.Unlock()
	for _, interval := range lw.intervals {
		// See https://golang.org/pkg/bytes/#Buffer.Write
		// The return value n is the length of p; err is always nil. If the
		// buffer becomes too large, Write will panic with ErrTooLarge.
		interval.Write(buf) // nolint: err
	}

	// Everything in lw.buf, now exists in each record, so we reset the buffer
	// so it does not grow too massive
	lw.buf.Reset()
}

// Adds a new interval which will be written to on each drain.
func (lw *IntervalRecorder) addInterval(interval *Interval) {
	lw.intervalsMu.Lock()
	defer lw.intervalsMu.Unlock()

	lw.intervals = append(lw.intervals, interval)
}

// Removes an interval so that it no longer receive data.
func (lw *IntervalRecorder) removeInterval(interval *Interval) {
	lw.intervalsMu.Lock()
	defer lw.intervalsMu.Unlock()

	for i, v := range lw.intervals {
		if interval == v {
			lw.intervals = append(lw.intervals[:i], lw.intervals[i+1:]...)
			break
		}
	}
}

// Start creates a new Interval, all data written to the IntervalRecorder
// after Start returns will be available in the interval. The interval should
// not be read from until Stop is called.
// See Stop for more detail
func (lw *IntervalRecorder) Start() *Interval {
	lw.drain()

	interval := &Interval{}
	interval.done = func() {
		lw.drain()
		lw.removeInterval(interval)
	}

	lw.addInterval(interval)

	return interval
}

// Stop will stop any further data being written to the interval after it returns.
func (r *Interval) Stop() {
	r.done()
}
