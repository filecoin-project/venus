package series

import (
	"context"
	"time"

	"github.com/filecoin-project/venus/internal/pkg/clock"
)

type ctxSleepDelayKey struct{}

var (
	// Key used to set the time.Duration in the context
	sleepDelayKey = ctxSleepDelayKey{}

	// Default delay
	defaultSleepDelay = clock.DefaultEpochDuration
)

// SetCtxSleepDelay returns a context with `d` set in the context. To sleep with
// the value, call CtxSleepDelay with the context.
func SetCtxSleepDelay(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, sleepDelayKey, d)
}

// CtxSleepDelay is a helper method to make sure people don't call `time.Sleep`
// or `time.After` themselves in series. It will use the time.Duration in the
// context, or default to `clock.epochDuration` from the venus/mining package.
// A channel is return which will receive a time.Time value after the delay.
func CtxSleepDelay(ctx context.Context) <-chan time.Time {
	d, ok := ctx.Value(sleepDelayKey).(time.Duration)
	if !ok {
		d = defaultSleepDelay
	}

	return time.After(d)
}
