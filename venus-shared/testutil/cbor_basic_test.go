package testutil

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
)

func TestCborBasicForAddress(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 16; i++ {
		var src, dst address.Address
		opt := CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				assert.Equal(t, src, address.Undef, "empty address")
				assert.Equal(t, src, dst, "empty cid")
			},

			Provided: func() {
				assert.NotEqual(t, src, dst, "address value provided")
			},

			Marshaled: func(b []byte) {
				t.Logf("marshaled callback called with %d bytes", len(b))
			},

			Finished: func() {
				assert.Equal(t, src, dst)
				assert.NotEqual(t, src, address.Undef, "must not be address.Undef")
			},
		}

		CborErBasicTest(t, &src, &dst, opt)
	}
}

func TestCborBasicForIDAddress(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 16; i++ {
		var src, dst address.Address
		opt := CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				assert.Equal(t, src, address.Undef, "empty address")
				assert.Equal(t, src, dst, "empty cid")
			},

			ProvideOpts: []interface{}{
				IDAddressProvider(),
			},

			Provided: func() {
				assert.NotEqual(t, src, dst, "address value provided")
				assert.Equal(t, src.Protocol(), address.ID, "must be id address")
			},

			Finished: func() {
				assert.Equal(t, src, dst)
				assert.NotEqual(t, src, address.Undef, "must not be address.Undef")
			},
		}

		CborErBasicTest(t, &src, &dst, opt)
	}
}
