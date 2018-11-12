package limiter

import (
	"sync"
	"time"
)

// Time interface defines required time methods for the Limiter struct.
// Primarily used for testing
type Time interface {
	Until(time.Time) time.Duration
}

// Limiter is used to restrict access to a resources till a future time
type Limiter struct {
	addrsMu sync.Mutex
	// addrs maps an address to the time when it is allowed to make additional requests
	addrs map[string]time.Time
	// time
	time Time
}

// NewLimiter returns a new limiter
func NewLimiter(tm Time) *Limiter {
	l := &Limiter{}

	l.addrs = make(map[string]time.Time)
	l.time = tm

	return l
}

// Add limits value till a given time
func (l *Limiter) Add(addr string, t time.Time) {
	l.addrsMu.Lock()
	defer l.addrsMu.Unlock()
	l.addrs[addr] = t
}

// Ready checks to see if the time has expired. Returns a time.Duration
// for the time remaining till a true value will be returned
func (l *Limiter) Ready(addr string) (time.Duration, bool) {
	l.addrsMu.Lock()
	defer l.addrsMu.Unlock()

	return l.ready(addr)
}

func (l *Limiter) ready(addr string) (time.Duration, bool) {
	if t, ok := l.addrs[addr]; ok && l.time.Until(t) > 0 {
		return l.time.Until(t), false
	}

	return 0, true
}

// Clear removes value from the limiter. Any calls to Ready for the value
// will return true after a call to Clear for it.
func (l *Limiter) Clear(addr string) {
	l.addrsMu.Lock()
	defer l.addrsMu.Unlock()
	l.clear(addr)
}

func (l *Limiter) clear(addr string) {
	delete(l.addrs, addr)
}

// Clean removes all values which a call to Ready would return true
func (l *Limiter) Clean() {
	l.addrsMu.Lock()
	defer l.addrsMu.Unlock()

	for addr := range l.addrs {
		if _, ok := l.ready(addr); ok {
			l.clear(addr)
		}
	}
}
