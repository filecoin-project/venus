package types

import (
	"math/big"
	"strings"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/specs-actors/actors/abi"
	specsabi "github.com/filecoin-project/specs-actors/actors/abi"
	specsbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/polydawn/refmt/obj/atlas"
)

func init() {
	encoding.RegisterIpldCborType(attoFILAtlasEntry)
}

var attoFILAtlasEntry = atlas.BuildEntry(AttoFIL{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(a AttoFIL) ([]byte, error) {
			return a.MarshalBinary()
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (AttoFIL, error) {
			return NewAttoFILFromBytes(x)
		})).
	Complete()

var attoPower = 18
var tenToTheEighteen = big.NewInt(10).Exp(big.NewInt(10), big.NewInt(18), nil)

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
	xAsBigInt := big.NewInt(0).SetUint64(x)
	return NewAttoFIL(xAsBigInt.Mul(xAsBigInt, tenToTheEighteen))
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

// DivCeil returns the minimum number of times this value can be divided into smaller amounts
// such that none of the smaller amounts are greater than the given divisor.
// Equal to ceil(z/y) if AttoFIL could be fractional.
// If y is zero a panic will occur.
func DivCeil(z, y AttoFIL) AttoFIL {
	value, remainder := big.NewInt(0).DivMod(z.Int, y.Int, big.NewInt(0))

	if remainder.Cmp(big.NewInt(0)) == 0 {
		return NewAttoFIL(value)
	}

	return NewAttoFIL(big.NewInt(0).Add(value, big.NewInt(1)))
}
