package chain

import (
	"bytes"
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/stretchr/testify/require"
)

func TestBigIntSerializationRoundTrip(t *testing.T) {
	tf.UnitTest(t)
	testValues := []string{
		"0", "1", "10", "-10", "9999", "12345678901234567891234567890123456789012345678901234567890",
	}

	for _, v := range testValues {
		bi, err := BigFromString(v)
		if err != nil {
			t.Fatal(err)
		}

		buf := new(bytes.Buffer)
		if err := bi.MarshalCBOR(buf); err != nil {
			t.Fatal(err)
		}

		var out BigInt
		if err := out.UnmarshalCBOR(buf); err != nil {
			t.Fatal(err)
		}

		if BigCmp(out, bi) != 0 {
			t.Fatal("failed to round trip BigInt through cbor")
		}

	}
}

func TestBigIntParseErr(t *testing.T) {
	tf.UnitTest(t)
	testValues := []string{
		"a0", "1b", "10c", "-1d0", "9e999", "f12345678901234567891234567890123456789012345678901234567890",
	}

	for _, v := range testValues {
		_, err := BigFromString(v)
		require.Error(t, err, "from invalid big int string")
	}
}

func TestBigIntCalculating(t *testing.T) {
	tf.UnitTest(t)
	zero := NewInt(0)
	maxProvideAttempts := 8
	for i := 0; i < 32; i++ {
		var a, b BigInt
		for attempt := 0; ; i++ {
			if attempt == maxProvideAttempts {
				t.Fatal("unable to get required numbers")
			}

			testutil.Provide(t, &a)
			testutil.Provide(t, &b)

			if a == EmptyInt || b == EmptyInt {
				t.Fatal("BigInt not provided")
			}

			if !a.Equals(zero) || !b.Equals(zero) {
				break
			}
		}

		sum := BigAdd(a, b)
		product := BigMul(a, b)

		require.True(t, BigSub(sum, a).Equals(b))
		require.True(t, BigDiv(product, a).Equals(b))

		base := a
		if base.IsZero() {
			base = b
		}

		base4 := BigMul(base, NewInt(4))
		require.Equal(t, BigDivFloat(base4, base), 4.0)
		require.Equal(t, BigDivFloat(base, base4), 0.25)

		abs := base.Abs()
		require.True(t, BigMod(abs, BigAdd(abs, NewInt(1))).Equals(abs))
	}
}
