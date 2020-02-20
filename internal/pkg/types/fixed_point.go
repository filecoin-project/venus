package types

// The fixed point type is an alias for uint64.  The type has two functions,
// one for converting from uint64 to a *big.Float and another for converting
// from a *big.Float to a uint64.  The decimal point is fixed 3 decimal digits
// from the least significant digit of the underlying uint64.  Ex:
//
// fixed point value: 1244590
// floating point value: 124.590
//
// The max integral number this implementation supports is 2^54 -1.
// This is chosen because ceil(log2(1000)) == 10, so we take 10 binary digits to
// encode the fractional part which leaves 54 bits for the integral part.

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
)

// MaxFixedPointIntegralNum is the largest whole number that can be encoded
// as a fixed point.
const MaxFixedPointIntegralNum = 18014398509481983 // (2^54 - 1)

// BigToFixed takes in a big Float and returns a uint64 encoded fixed point.
func BigToFixed(f *big.Float) (fbig.Int, error) {
	// check that f is not too big.
	if cmp := f.Cmp(big.NewFloat(float64(MaxFixedPointIntegralNum))); cmp == 1 {
		return fbig.Zero(), errors.New("float too big to store in fixed point")
	}

	s := fmt.Sprintf("%.3f", f) // nolint: govet
	parts := strings.Split(s, ".")
	integral, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return fbig.Zero(), err
	}
	fractional, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return fbig.Zero(), err
	}

	fixed := integral * 1000
	fixed += fractional
	return Uint64ToBig(fixed), nil
}

// FixedToBig takes in a uint64 encoded fixed point and returns a big Float.
func FixedToBig(fixed fbig.Int) (*big.Float, error) {
	uFixed, err := BigToUint64(fixed)
	if err != nil {
		return nil, err
	}

	q := int64(uFixed / 1000)
	m := int64(uFixed % 1000)

	integral := big.NewFloat(0.0).SetInt64(q)
	integral.SetPrec(64)
	fractional := big.NewFloat(0.0).SetInt64(m)

	divF := big.NewFloat(1000.0)
	fractional.Quo(fractional, divF)

	integral.Add(integral, fractional)
	return integral, nil
}

// FixedStr returns a printable string with the correct decimal place for the
// input uint64 encoded fixed point number.
func FixedStr(fixed fbig.Int) (string, error) {
	b, err := FixedToBig(fixed)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%.3f", b), nil // nolint: govet
}

// BigToUint64 converts a big Int to a uint64.  It will error if
// the Int is too big to fit into 64 bits or is negative
func BigToUint64(bi fbig.Int) (uint64, error) {
	if !bi.Int.IsUint64() {
		return 0, fmt.Errorf("Int: %s could not be represented as uint64", bi.String())
	}
	return bi.Uint64(), nil
}

// Uint64ToBig converts a uint64 to a big Int.  Precodition: don't overflow int64.
func Uint64ToBig(u uint64) fbig.Int {
	return fbig.NewInt(int64(u))
}
