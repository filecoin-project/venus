package clock

import (
	"time"
)

// Timer provides an interface to a time.Timer which is testable.
// See https://golang.org/pkg/time/#Timer for more details on how timers work.
type Timer interface {
	Chan() <-chan time.Time
	Reset(d time.Duration) bool
	Stop() bool
}

type realTimer struct {
	*time.Timer
}

func (rt *realTimer) Chan() <-chan time.Time {
	return rt.C
}
