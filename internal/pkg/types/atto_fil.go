package types

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/filecoin-project/go-state-types/abi"
	specsabi "github.com/filecoin-project/go-state-types/abi"
	specsbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
)

var attoPower = 18
var tenToTheEighteen = specsbig.Exp(specsbig.NewInt(10), specsbig.NewInt(18))

// ZeroAttoFIL is the zero value for an AttoFIL, exported for consistency in construction of AttoFILs
var ZeroAttoFIL = specsbig.Zero()

// AttoFIL represents a signed multi-precision integer quantity of
// attofilecoin (atto is metric for 10**-18). The zero value for
// AttoFIL represents the value 0.
//
// Reasons for embedding a big.Int instead of *big.Int:
//   - We don't have check for nil in every method that does calculations.
//   - Serialization "symmetry" when serializing AttoFIL{}.
type AttoFIL = specsabi.TokenAmount

// NewAttoFIL allocates and returns a new AttoFIL set to x.
func NewAttoFIL(x *big.Int) AttoFIL {
	return specsbig.Int{Int: x}
}

// NewAttoFILFromFIL returns a new AttoFIL representing a quantity
// of attofilecoin equal to x filecoin.
func NewAttoFILFromFIL(x uint64) AttoFIL {
	xAsBigInt := specsbig.NewIntUnsigned(x)
	return specsbig.Mul(xAsBigInt, tenToTheEighteen)
}

var tenToTheEighteenTokens = specsbig.Exp(specsbig.NewInt(10), specsbig.NewInt(18))

// NewAttoTokenFromToken should be moved when we cleanup the types
// Dragons: clean up and likely move to specs-actors
func NewAttoTokenFromToken(x uint64) abi.TokenAmount {
	xAsBigInt := abi.NewTokenAmount(0)
	xAsBigInt.SetUint64(x)
	return specsbig.Mul(xAsBigInt, tenToTheEighteenTokens)
}

// NewAttoFILFromBytes allocates and returns a new AttoFIL set
// to the value of buf as the bytes of a big-endian unsigned integer.
func NewAttoFILFromBytes(buf []byte) (AttoFIL, error) {
	var af AttoFIL
	err := encoding.Decode(buf, &af)
	if err != nil {
		return af, err
	}
	return af, nil
}

// NewAttoFILFromFILString allocates a new AttoFIL set to the value of s filecoin,
// interpreted as a decimal in base 10, and returns it and a boolean indicating success.
func NewAttoFILFromFILString(s string) (AttoFIL, bool) {
	splitNumber := strings.Split(s, ".")
	// If '.' is absent from string, add an empty string to become the decimal part
	if len(splitNumber) == 1 {
		splitNumber = append(splitNumber, "")
	}
	intPart := splitNumber[0]
	decPart := splitNumber[1]
	// A decimal part longer than 18 digits should be an error
	if len(decPart) > attoPower || len(splitNumber) > 2 {
		return ZeroAttoFIL, false
	}
	// The decimal is right padded with 0's if it less than 18 digits long
	for len(decPart) < attoPower {
		decPart += "0"
	}

	return NewAttoFILFromString(intPart+decPart, 10)
}

// NewAttoFILFromString allocates a new AttoFIL set to the value of s attofilecoin,
// interpreted in the given base, and returns it and a boolean indicating success.
func NewAttoFILFromString(s string, base int) (AttoFIL, bool) {
	out := specsbig.NewInt(0)
	_, isErr := out.Int.SetString(s, base)
	return out, isErr
}

// BigToUint64 converts a big Int to a uint64.  It will error if
// the Int is too big to fit into 64 bits or is negative
func BigToUint64(bi specsbig.Int) (uint64, error) {
	if !bi.Int.IsUint64() {
		return 0, fmt.Errorf("Int: %s could not be represented as uint64", bi.String())
	}
	return bi.Uint64(), nil
}

// Uint64ToBig converts a uint64 to a big Int.  Precodition: don't overflow int64.
func Uint64ToBig(u uint64) specsbig.Int {
	return specsbig.NewInt(int64(u))
}
