package types

import (
	"math/big"
	"strings"

	specsbig "github.com/filecoin-project/go-state-types/big"
)

var attoPower = 18
var tenToTheEighteen = specsbig.Exp(specsbig.NewInt(10), specsbig.NewInt(18))

// NewAttoFIL allocates and returns a new AttoFIL set to x.
func NewAttoFIL(x *big.Int) specsbig.Int {
	return specsbig.Int{Int: x}
}

// NewAttoFILFromFIL returns a new AttoFIL representing a quantity
// of attofilecoin equal to x filecoin.
func NewAttoFILFromFIL(x uint64) specsbig.Int {
	xAsBigInt := specsbig.NewIntUnsigned(x)
	return specsbig.Mul(xAsBigInt, tenToTheEighteen)
}

// NewAttoFILFromFILString allocates a new AttoFIL set to the value of s filecoin,
// interpreted as a decimal in base 10, and returns it and a boolean indicating success.
func NewAttoFILFromFILString(s string) (specsbig.Int, bool) {
	splitNumber := strings.Split(s, ".")
	// If '.' is absent from string, add an empty string to become the decimal part
	if len(splitNumber) == 1 {
		splitNumber = append(splitNumber, "")
	}
	intPart := splitNumber[0]
	decPart := splitNumber[1]
	// A decimal part longer than 18 digits should be an error
	if len(decPart) > attoPower || len(splitNumber) > 2 {
		return ZeroFIL, false
	}
	// The decimal is right padded with 0's if it less than 18 digits long
	for len(decPart) < attoPower {
		decPart += "0"
	}

	return NewAttoFILFromString(intPart+decPart, 10)
}

// NewAttoFILFromString allocates a new AttoFIL set to the value of s attofilecoin,
// interpreted in the given base, and returns it and a boolean indicating success.
func NewAttoFILFromString(s string, base int) (specsbig.Int, bool) {
	out := specsbig.NewInt(0)
	_, isErr := out.Int.SetString(s, base)
	return out, isErr
}
