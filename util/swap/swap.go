package swap

import (
	"reflect"

	"github.com/filecoin-project/go-filecoin/util/chk"
)

// Swap replaces a destination value with a new value and returns a function
// that can be used to undo the swap. This can be used with defer to
// automatically unwind changes at the end of a scope:
//
// `defer Swap(foo, bar)()`
//
// The destination value must be a pointer and the new value must be a value
// that can be assigned to the destination pointer.
func Swap(dst, new interface{}) (unswap func()) {
	dstrv := reflect.ValueOf(dst)
	newrv := reflect.ValueOf(new)
	chk.Equal(reflect.Ptr, dstrv.Kind(), "Destination of swap must be a pointer")

	oldrv := reflect.Indirect(dstrv)
	chk.True(oldrv.CanSet(), "Destination of swap must be settable")

	snapshot := oldrv.Interface()
	oldrv.Set(newrv)

	return func() {
		reflect.Indirect(dstrv).Set(reflect.ValueOf(snapshot))
	}
}
