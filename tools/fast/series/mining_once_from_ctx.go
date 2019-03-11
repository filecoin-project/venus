package series

import (
	"context"
)

// CKMiningOnce is the key used to store the MiningOnceFunc in a context
var CKMiningOnce = struct{}{}

// MiningOnceFunc is the type for the value stored under the CKMiningOnce key
type MiningOnceFunc func()

// MiningOnceFromCtx extracts the MiningOnceFunc through key CKMiningOnce
func MiningOnceFromCtx(ctx context.Context) {
	MiningOnce, ok := ctx.Value(CKMiningOnce).(MiningOnceFunc)
	if ok {
		MiningOnce()
	}
}
