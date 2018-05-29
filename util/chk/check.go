// Package chk includes some very basic utilities for invariant checking.
//
// These functions are purposely limited to avoid allocations, so that they
// can be used even in performance-sensitive code. They only allocate if
// you use the formatting features. So:
//
// True(bonk)  // alloc-free
// True(bonk, "foo")  // alloc-free if "foo" is constant expression (*)
// True(bonk, "foo %s", arg) // allocates
//
// (*) https://groups.google.com/d/msg/golang-nuts/TK0rPdxgCPk/H6roZygkVccJ
package chk

import "fmt"

// True panics if a boolean condition is not true.
func True(cond bool, formatAndArgs ...interface{}) {
	if !cond {
		var msg string
		if len(formatAndArgs) == 0 {
			msg = "invariant violation"
		} else {
			msg = args(formatAndArgs[0], formatAndArgs[1:]...)
		}

		panic(msg)
	}
}

// False panics is a boolean condition is true.
func False(cond bool, formatAndArgs ...interface{}) {
	True(!cond, formatAndArgs...)
}

func args(format interface{}, args ...interface{}) string {
	var formatStr string
	var ok bool
	if formatStr, ok = format.(string); !ok {
		formatStr = fmt.Sprintf("%v", format)
	}
	return fmt.Sprintf(formatStr, args...)
}
