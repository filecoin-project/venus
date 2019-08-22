package series

import (
	"context"
)

type ctxMiningOnceKey struct{}
type ctxMessageWaitKey struct{}

// Key used to store the MiningOnceFunc in the context
var miningOnceKey = ctxMiningOnceKey{}

// MiningOnceFunc is the type for the value used when calling SetCtxMiningOnce
type MiningOnceFunc func()

// Key used to store the MpoolWaitFunc in the context
var mpoolWaitKey = ctxMessageWaitKey{}

// MpoolWaitFunc is a function that can wait for a message to appear in its queu
type MpoolWaitFunc func()

// SetCtxMiningOnce returns a context with `fn` set in the context. To run the
// MiningOnceFunc value, call CtxMiningOnce.
func SetCtxMiningOnce(ctx context.Context, fn MiningOnceFunc) context.Context {
	return context.WithValue(ctx, miningOnceKey, fn)
}

// SetCtxWaitForMpool returns a context with `fn` set in the context. To run the
// MiningOnceFunc with a MpoolWaitFunc value, call CtxMiningOnceForBlockingCommand.
func SetCtxWaitForMpool(ctx context.Context, fn MpoolWaitFunc) context.Context {
	return context.WithValue(ctx, mpoolWaitKey, fn)
}

// CtxMiningOnce will call the MiningOnceFunc set on the context using
// SetMiningOnceFunc. If no value is set on the context, the call is a noop.
func CtxMiningOnce(ctx context.Context) {
	miningOnce, ok := ctx.Value(miningOnceKey).(MiningOnceFunc)
	if ok {
		miningOnce()
	}
}

// CtxMiningNext will call CtxMiningOnce only after a message is in the message pool ready to mine.
// This lets us run blocking commands that require mining by configuring the mining beforehand.
func CtxMiningNext(ctx context.Context, count int) {
	mpoolWait, ok := ctx.Value(mpoolWaitKey).(MpoolWaitFunc)
	if !ok {
		panic("series not configured with message wait function")
	}

	miningOnce, ok := ctx.Value(miningOnceKey).(MiningOnceFunc)
	if !ok {
		panic("series not configured with mining once function")
	}

	go func() {
		for i := 0; i < count; i++ {
			// wait for one message then mine it
			mpoolWait()
			miningOnce()
		}
	}()
}
