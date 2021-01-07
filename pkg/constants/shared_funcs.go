package constants

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/filecoin-project/go-address"
	big2 "github.com/filecoin-project/go-state-types/big"
)

func MustParseAddress(addr string) address.Address {
	ret, err := address.NewFromString(addr)
	if err != nil {
		panic(err)
	}

	return ret
}

func ParseFIL(s string) (big2.Int, error) {
	suffix := strings.TrimLeft(s, ".1234567890")
	s = s[:len(s)-len(suffix)]
	var attofil bool
	if suffix != "" {
		norm := strings.ToLower(strings.TrimSpace(suffix))
		switch norm {
		case "", "fil":
		case "attofil", "afil":
			attofil = true
		default:
			return big2.Zero(), fmt.Errorf("unrecognized suffix: %q", suffix)
		}
	}

	if len(s) > 50 {
		return big2.Zero(), fmt.Errorf("string length too large: %d", len(s))
	}

	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return big2.Zero(), fmt.Errorf("failed to parse %q as a decimal number", s)
	}

	if !attofil {
		r = r.Mul(r, big.NewRat(int64(FilecoinPrecision), 1))
	}

	if !r.IsInt() {
		var pref string
		if attofil {
			pref = "atto"
		}
		return big2.Zero(), fmt.Errorf("invalid %sFIL value: %q", pref, s)
	}

	return big2.NewFromGo(r.Num()), nil
}

func MustParseFIL(s string) big2.Int {
	n, err := ParseFIL(s)
	if err != nil {
		panic(err)
	}

	return n
}
