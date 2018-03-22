package types

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var hashes = make([][]byte, 5)

func init() {
	for i := range hashes {
		s, err := AddressHash([]byte(fmt.Sprintf("foo-%d", i)))
		if err != nil {
			panic(err)
		}
		hashes[i] = s
	}
}

func TestNewAddress(t *testing.T) {
	assert := assert.New(t)

	a := NewMainnetAddress(hashes[0])
	assert.Len(a.Hash(), 20)
	assert.Equal(a.Hash(), hashes[0])

	assert.Equal(a.Network(), Mainnet)
	assert.Equal(a.Version(), uint8(0))
	assert.Len(a.String(), 41)
}

func TestValidAddresses(t *testing.T) {
	testCases := []struct {
		input  Address
		output string
	}{
		{NewMainnetAddress(hashes[0]), "fcqeutlg2sl9daptdcfm8sw7m3xzd0tqhz8f4nzc9"},
		{NewMainnetAddress(hashes[1]), "fcqwfptkd8ax6xqg7tvycd9wkfyg748fqjwnlt9a0"},
		{NewTestnetAddress(hashes[0]), "tfqeutlg2sl9daptdcfm8sw7m3xzd0tqhz8g9f95l"},
		{NewTestnetAddress(hashes[1]), "tfqwfptkd8ax6xqg7tvycd9wkfyg748fqjwj03z34"},
	}

	for _, tc := range testCases {
		t.Run(tc.input.String(), func(t *testing.T) {
			assert := assert.New(t)
			assert.Equal(tc.input.String(), tc.output)

			a, err := NewAddressFromString(tc.input.String())
			assert.NoError(err)
			assert.Equal(a, tc.input)

			assert.NoError(ParseError(a.String()))
			assert.NoError(ParseError(tc.output))
			assert.NoError(ParseError(tc.input.String()))
		})

		t.Run(fmt.Sprintf("roundtrip bytes: %s", tc.input), func(t *testing.T) {
			assert := assert.New(t)

			a, err := NewAddressFromBytes(tc.input.Bytes())
			assert.NoError(err)
			assert.Equal(a, tc.input)
		})
	}
}

func TestInvalidAddresses(t *testing.T) {
	// TODO: write me
}

func TestAddressFormat(t *testing.T) {
	assert := assert.New(t)

	a := MakeTestAddress("hello")
	assert.Equal(fmt.Sprintf(" %s", a), " tfqk4f3cuph7pkf7228zv4x5aeq9scgazfewaq2he")
	assert.Equal(fmt.Sprintf("%X", a), "0100B5531C7037F06C9F2947132A6A77202C308E8939")
	assert.Equal(fmt.Sprintf("%v", a), "[tf - 0 - b5531c7037f06c9f2947132a6a77202c308e8939]")
}

func TestChecksum(t *testing.T) {
	assert := assert.New(t)

	hrp := "hi"
	data := []byte("helloworld")

	checksum := createChecksum(hrp, data)
	assert.Len(checksum, 6)

	combined := append(data, []byte(checksum)...)
	assert.True(verifyChecksum(hrp, combined))
}

func TestAddressJSON(t *testing.T) {
	assert := assert.New(t)

	a := MakeTestAddress("first")

	out, err := json.Marshal(a)
	assert.NoError(err)
	assert.Equal(string(out), fmt.Sprintf(`"%s"`, a.String()))

	var b Address
	assert.NoError(json.Unmarshal(out, &b))
	assert.Equal(a, b)
}
