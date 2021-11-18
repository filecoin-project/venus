package testutil

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/require"
)

func TestCborBasicForAddress(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 16; i++ {
		var src, dst address.Address
		opt := CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, address.Undef, "empty address")
				require.Equal(t, src, dst, "empty cid")
			},

			Provided: func() {
				require.NotEqual(t, src, dst, "address value provided")
			},

			Marshaled: func(b []byte) {
				t.Logf("marshaled callback called with %d bytes", len(b))
			},

			Finished: func() {
				require.Equal(t, src, dst)
				require.NotEqual(t, src, address.Undef, "must not be address.Undef")
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
				require.Equal(t, src, address.Undef, "empty address")
				require.Equal(t, src, dst, "empty cid")
			},

			ProvideOpts: []interface{}{
				IDAddressProvider(),
			},

			Provided: func() {
				require.NotEqual(t, src, dst, "address value provided")
				require.Equal(t, src.Protocol(), address.ID, "must be id address")
			},

			Finished: func() {
				require.Equal(t, src, dst)
				require.NotEqual(t, src, address.Undef, "must not be address.Undef")
			},
		}

		CborErBasicTest(t, &src, &dst, opt)
	}
}
