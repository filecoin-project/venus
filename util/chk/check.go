// Package chk includes some very basic utilities for invariant checking.
//
// Functionality here is purposely limited to help avoid mistakes. Features
// like deep equality checking with reflection and coercion are omitted to
// encourage precision at the callsite.
//
// These functions are designed to be pay-for-what-you-They will only
// allocate if arguments beyond the first format parameter are passed.
// So, for performance critical tight loops, you can do True(bonk, "foo")
// and that should be alloc free. For other places, you can use the more
// expressive `True(bonk, "foo: %s", "bar")`, which may allocate.
package chk

import "fmt"

// True panics if a boolean condition is not true.
func True(cond bool, formatAndArgs ...interface{}) {
	var msg string
	if len(formatAndArgs) == 0 {
		msg = "invariant violation"
	} else {
		msg = args(formatAndArgs[0], formatAndArgs[1:]...)
	}

	if !cond {
		panic(msg)
	}
}

// False panics is a boolean condition is true.
func False(cond bool, formatAndArgs ...interface{}) {
	True(!cond, formatAndArgs...)
}

// Nil panics if a thing is not nil.
func Nil(thing interface{}, formatAndArgs ...interface{}) {
	True(thing == nil, formatAndArgs...)
}

// NotNil panics if a thing is nil.
func NotNil(thing interface{}, formatAndArgs ...interface{}) {
	True(thing != nil, formatAndArgs...)
}

// Equal panics if two values are not equal. Equality here is in the
// sense of Go's == operator.
func Equal(expected, actual interface{}, formatAndArgs ...interface{}) {
	msg := fmt.Sprintf("expected: %v - actual: %v", expected, actual)
	if len(formatAndArgs) > 0 {
		msg += " - " + args(formatAndArgs[0], formatAndArgs...)
	}
	True(expected == actual)
}

// NotEqual panics if two objects are equal. Equality here is in the sense
// of Go's == operator.
func NotEqual(expected, actual interface{}, formatAndArgs ...interface{}) {
	True(expected != actual, formatAndArgs...)
}

func args(format interface{}, args ...interface{}) string {
	var formatStr string
	var ok bool
	if formatStr, ok = format.(string); !ok {
		format = fmt.Sprintf("%v", format)
	}
	return fmt.Sprintf(formatStr, args...)
}
