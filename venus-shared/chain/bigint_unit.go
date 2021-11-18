package chain

import (
	"fmt"
	"math/big"
)

var byteSizeUnits = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB"}

func SizeStr(bi BigInt) string {
	f, i := unitNumber(bi, byteSizeUnits)
	return fmt.Sprintf("%.4g %s", f, byteSizeUnits[i])
}

var deciUnits = []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi"}

func DeciStr(bi BigInt) string {
	f, i := unitNumber(bi, deciUnits)
	return fmt.Sprintf("%.3g %s", f, deciUnits[i])
}

func unitNumber(n BigInt, units []string) (float64, int) {
	r := new(big.Rat).SetInt(n.Int)
	den := big.NewRat(1, 1024)

	var i int
	for f, _ := r.Float64(); f >= 1024 && i+1 < len(units); f, _ = r.Float64() {
		i++
		r = r.Mul(r, den)
	}

	f, _ := r.Float64()
	return f, i
}
