package series

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
)

type ctxSleepDelayKey struct{}

var (
	// Key used to set the time.Duration in the context
	sleepDelayKey = ctxSleepDelayKey{}

	// Default delay
	defaultSleepDelay = consensus.DefaultBlockTime
)

// SetCtxSleepDelay returns a context with `d` set in the context. To sleep with
// the value, call CtxSleepDelay with the context.
func SetCtxSleepDelay(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, sleepDelayKey, d)
}

// CtxSleepDelay is a helper method to make sure people don't call `time.Sleep`
// or `time.After` themselves in series. It will use the time.Duration in the
// context, or default to `mining.DefaultBlockTime` from the go-filecoin/mining package.
// A channel is return which will receive a time.Time value after the delay.
func CtxSleepDelay(ctx context.Context) <-chan time.Time {
	d, ok := ctx.Value(sleepDelayKey).(time.Duration)
	if !ok {
		d = defaultSleepDelay
	}

	return time.After(d)
}
