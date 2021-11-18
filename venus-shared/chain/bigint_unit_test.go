package chain

import (
	"math/big"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-units"

	"github.com/stretchr/testify/require"
)

func TestUnitStrs(t *testing.T) {
	cases := []struct {
		in   uint64
		size string
		deci string
	}{
		{0, "0 B", "0 "},
		{1, "1 B", "1 "},
		{1016, "1016 B", "1.02e+03 "},
		{1024, "1 KiB", "1 Ki"},
		{1000 * 1024, "1000 KiB", "1e+03 Ki"},
		{2000, "1.953 KiB", "1.95 Ki"},
		{5 << 20, "5 MiB", "5 Mi"},
		{11 << 60, "11 EiB", "11 Ei"},
	}

	for _, c := range cases {
		require.Equal(t, c.size, SizeStr(NewInt(c.in)), "result of SizeStr")
		require.Equal(t, c.deci, DeciStr(NewInt(c.in)), "result of DeciStr")
	}
}

func TestSizeStrUnitsSymmetry(t *testing.T) {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)

	for i := 0; i < 10000; i++ {
		n := r.Uint64()
		l := strings.ReplaceAll(units.BytesSize(float64(n)), " ", "")
		r := strings.ReplaceAll(SizeStr(NewInt(n)), " ", "")

		require.NotContains(t, l, "e+")
		require.NotContains(t, r, "e+")

		require.Equal(t, l, r, "wrong formatting for %d", n)
	}
}

func TestSizeStrBig(t *testing.T) {
	ZiB := big.NewInt(50000)
	ZiB = ZiB.Lsh(ZiB, 70)

	require.Equal(t, "5e+04 ZiB", SizeStr(BigInt{Int: ZiB}), "inout %+v, produced wrong result", ZiB)

}
