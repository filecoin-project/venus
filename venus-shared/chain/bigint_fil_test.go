package chain

import (
	"strings"
	"testing"

	"github.com/filecoin-project/venus/venus-shared/chain/params"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilRoundTrip(t *testing.T) {
	testValues := []string{
		"0 FIL", "1 FIL", "1.001 FIL", "100.10001 FIL", "101100 FIL", "5000.01 FIL", "5000 FIL",
		strings.Repeat("1", 50) + " FIL",
	}

	for _, v := range testValues {
		fval := MustParseFIL(v)

		if fval.String() != v {
			t.Fatal("mismatch in values!", v, fval.String())
		}

		text, err := fval.MarshalText()
		assert.NoError(t, err, "marshal text for fval")

		fval2 := FIL(NewInt(0))
		err = fval2.UnmarshalText(text)
		assert.NoError(t, err, "unmarshal text for fval2")
		assert.True(t, BigInt{Int: fval.Int}.Equals(BigInt{Int: fval2.Int}))
	}
}

func TestParseAttoFils(t *testing.T) {
	testValues := []string{
		"0 aFIL", "1 aFIL", "1 aFIL", "100 aFIL", "101100 aFIL", "5000 aFIL",
		"0 attoFIL", "1 attoFIL", "1 attoFIL", "100 attoFIL", "101100 attoFIL", "5000 attoFIL",
	}

	for _, v := range testValues {
		fval := MustParseFIL(v)

		text, err := fval.MarshalText()
		assert.NoError(t, err, "marshal text for fval")

		fval2 := FIL(NewInt(0))
		err = fval2.UnmarshalText(text)
		assert.NoError(t, err, "unmarshal text for fval2")
		assert.True(t, BigInt{Int: fval.Int}.Equals(BigInt{Int: fval2.Int}))
	}
}

func TestInvalidFILString(t *testing.T) {
	testValues := []string{
		"0 nFIL", "1 nFIL", "1.001 nFIL", "100.10001 nFIL", "101100 nFIL", "5000.01 nFIL", "5000 nFIL",
		"1.001.1 FIL",
		strings.Repeat("1", 51) + " FIL",
	}

	for _, v := range testValues {
		_, err := ParseFIL(v)
		require.Errorf(t, err, "invalid fil string %s", v)
	}
}

func TestBigFromFIL(t *testing.T) {
	ratio := NewInt(params.FilecoinPrecision)

	nums := make([]uint64, 32)
	testutil.Provide(t, &nums, testutil.IntRangedProvider(10, 1000))

	for i := range nums {
		fval := FromFil(nums[i])
		require.True(t, fval.GreaterThan(ZeroFIL), "greater than zero")
		require.True(t, ratio.Equals(BigDiv(fval, NewInt(nums[i]))), "fil precision")
	}
}

func TestFilShort(t *testing.T) {
	for _, s := range []struct {
		fil    string
		expect string
	}{

		{fil: "1", expect: "1 FIL"},
		{fil: "1.1", expect: "1.1 FIL"},
		{fil: "12", expect: "12 FIL"},
		{fil: "123", expect: "123 FIL"},
		{fil: "123456", expect: "123456 FIL"},
		{fil: "123.23", expect: "123.23 FIL"},
		{fil: "123456.234", expect: "123456.234 FIL"},
		{fil: "123456.2341234", expect: "123456.234 FIL"},
		{fil: "123456.234123445", expect: "123456.234 FIL"},

		{fil: "0.1", expect: "100 mFIL"},
		{fil: "0.01", expect: "10 mFIL"},
		{fil: "0.001", expect: "1 mFIL"},

		{fil: "0.0001", expect: "100 μFIL"},
		{fil: "0.00001", expect: "10 μFIL"},
		{fil: "0.000001", expect: "1 μFIL"},

		{fil: "0.0000001", expect: "100 nFIL"},
		{fil: "0.00000001", expect: "10 nFIL"},
		{fil: "0.000000001", expect: "1 nFIL"},

		{fil: "0.0000000001", expect: "100 pFIL"},
		{fil: "0.00000000001", expect: "10 pFIL"},
		{fil: "0.000000000001", expect: "1 pFIL"},

		{fil: "0.0000000000001", expect: "100 fFIL"},
		{fil: "0.00000000000001", expect: "10 fFIL"},
		{fil: "0.000000000000001", expect: "1 fFIL"},

		{fil: "0.0000000000000001", expect: "100 aFIL"},
		{fil: "0.00000000000000001", expect: "10 aFIL"},
		{fil: "0.000000000000000001", expect: "1 aFIL"},

		{fil: "0.0000012", expect: "1.2 μFIL"},
		{fil: "0.00000123", expect: "1.23 μFIL"},
		{fil: "0.000001234", expect: "1.234 μFIL"},
		{fil: "0.0000012344", expect: "1.234 μFIL"},
		{fil: "0.00000123444", expect: "1.234 μFIL"},

		{fil: "0.0002212", expect: "221.2 μFIL"},
		{fil: "0.00022123", expect: "221.23 μFIL"},
		{fil: "0.000221234", expect: "221.234 μFIL"},
		{fil: "0.0002212344", expect: "221.234 μFIL"},
		{fil: "0.00022123444", expect: "221.234 μFIL"},

		{fil: "-1", expect: "-1 FIL"},
		{fil: "-1.1", expect: "-1.1 FIL"},
		{fil: "-12", expect: "-12 FIL"},
		{fil: "-123", expect: "-123 FIL"},
		{fil: "-123456", expect: "-123456 FIL"},
		{fil: "-123.23", expect: "-123.23 FIL"},
		{fil: "-123456.234", expect: "-123456.234 FIL"},
		{fil: "-123456.2341234", expect: "-123456.234 FIL"},
		{fil: "-123456.234123445", expect: "-123456.234 FIL"},

		{fil: "-0.1", expect: "-100 mFIL"},
		{fil: "-0.01", expect: "-10 mFIL"},
		{fil: "-0.001", expect: "-1 mFIL"},

		{fil: "-0.0001", expect: "-100 μFIL"},
		{fil: "-0.00001", expect: "-10 μFIL"},
		{fil: "-0.000001", expect: "-1 μFIL"},

		{fil: "-0.0000001", expect: "-100 nFIL"},
		{fil: "-0.00000001", expect: "-10 nFIL"},
		{fil: "-0.000000001", expect: "-1 nFIL"},

		{fil: "-0.0000000001", expect: "-100 pFIL"},
		{fil: "-0.00000000001", expect: "-10 pFIL"},
		{fil: "-0.000000000001", expect: "-1 pFIL"},

		{fil: "-0.0000000000001", expect: "-100 fFIL"},
		{fil: "-0.00000000000001", expect: "-10 fFIL"},
		{fil: "-0.000000000000001", expect: "-1 fFIL"},

		{fil: "-0.0000000000000001", expect: "-100 aFIL"},
		{fil: "-0.00000000000000001", expect: "-10 aFIL"},
		{fil: "-0.000000000000000001", expect: "-1 aFIL"},

		{fil: "-0.0000012", expect: "-1.2 μFIL"},
		{fil: "-0.00000123", expect: "-1.23 μFIL"},
		{fil: "-0.000001234", expect: "-1.234 μFIL"},
		{fil: "-0.0000012344", expect: "-1.234 μFIL"},
		{fil: "-0.00000123444", expect: "-1.234 μFIL"},

		{fil: "-0.0002212", expect: "-221.2 μFIL"},
		{fil: "-0.00022123", expect: "-221.23 μFIL"},
		{fil: "-0.000221234", expect: "-221.234 μFIL"},
		{fil: "-0.0002212344", expect: "-221.234 μFIL"},
		{fil: "-0.00022123444", expect: "-221.234 μFIL"},
	} {
		s := s
		t.Run(s.fil, func(t *testing.T) {
			f, err := ParseFIL(s.fil)
			require.NoError(t, err)
			require.Equal(t, s.expect, f.Short())
		})
	}
}
