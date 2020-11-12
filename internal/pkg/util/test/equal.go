package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

// Equal asserts that two objects are equal.
//
//    assert.Equal(t, 123, 123)
//
// Pointer variable equality is determined based on the equality of the
// referenced values (as opposed to the memory addresses). Function equality
// cannot be determined and will always fail.
func Equal(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	expectRaw, err := json.Marshal(expected)
	if err != nil {
		return false
	}
	actualRaw, err := json.Marshal(actual)
	if err != nil {
		return false
	}

	if !bytes.Equal(expectRaw, actualRaw) {
		return assert.Fail(t, fmt.Sprintf("Not equal: \n"+
			"expected: %s\n"+
			"actual  : %s", string(expectRaw), string(actualRaw)), msgAndArgs...)
	}
	return true
}
