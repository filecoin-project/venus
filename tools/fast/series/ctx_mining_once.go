package series

import (
	"context"
	"errors"
)

type ctxMiningOnceKey struct{}

// Key used to store the MiningOnceFunc in the context
var miningOnceKey = ctxMiningOnceKey{}

// MiningOnceFunc is the type for the value used when calling SetCtxMiningOnce
type MiningOnceFunc func() error

// SetCtxMiningOnce returns a context with `fn` set in the context. To run the
// MiningOnceFunc value, call CtxMiningOnce.
func SetCtxMiningOnce(ctx context.Context, fn MiningOnceFunc) context.Context {
	return context.WithValue(ctx, miningOnceKey, fn)
}

// CtxMiningOnce will call the MiningOnceFunc set on the context using
// SetMiningOnceFunc. If no value is set on the context, the call is a noop.
func CtxMiningOnce(ctx context.Context) error {
	miningOnce, ok := ctx.Value(miningOnceKey).(MiningOnceFunc)
	if ok {
		return miningOnce()
	}
	return errors.New("Not Set")
}
