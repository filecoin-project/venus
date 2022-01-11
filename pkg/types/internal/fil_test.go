package internal

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

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

func TestMarshal(t *testing.T) {
	type A struct {
		Fil FIL
	}
	var a = A{
		Fil: FIL{big.NewInt(1000000)},
	}

	aBytes, err := json.Marshal(a)
	require.NoError(t, err)

	require.Equal(t, aBytes, []byte("{\"Fil\":\"0.000000000001 FIL\"}"))
	fmt.Println(string(aBytes))
}

func TestUnMarshal(t *testing.T) {
	type A struct {
		Fil FIL
	}
	bigFIl, _ := big.NewInt(0).SetString("100000000000000000000", 10)
	for _, s := range []struct {
		fil    string
		expect FIL
	}{
		{
			fil:    "{\"Fil\":\"0.000000000001 FIL\"}",
			expect: FIL{big.NewInt(1000000)},
		},
		{
			fil:    "{\"Fil\":\"1 FIL\"}",
			expect: FIL{big.NewInt(1000000000000000000)},
		},
		{
			fil:    "{\"Fil\":\"100 FIL\"}",
			expect: FIL{bigFIl},
		},
	} {
		a := A{}
		err := json.Unmarshal([]byte(s.fil), &a)
		require.NoError(t, err)
		require.Equal(t, a.Fil.String(), s.expect.String())
	}
}
