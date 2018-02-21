package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddressToString(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(Address("hello").String(), "0x68656c6c6f")
}

func TestParseAddress(t *testing.T) {
	testCases := []struct {
		error  error
		input  string
		output Address
	}{
		{nil, "0x68656c6c6f", Address("hello")},
		{fmt.Errorf("addresses must start with 0x, got 123"), "123", Address("")},
		{fmt.Errorf("decoding address failed: 0xxyz: encoding/hex: invalid byte: U+0078 'x'"), "0xxyz", Address("")},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("(%t) %s -> %s", tc.error != nil, tc.input, tc.output), func(t *testing.T) {
			assert := assert.New(t)

			expected, err := ParseAddress(tc.input)
			if tc.error == nil {
				assert.NoError(err)
			} else {
				assert.Equal(err.Error(), tc.error.Error())
				assert.Equal(expected, tc.output)
			}
		})
	}
}
