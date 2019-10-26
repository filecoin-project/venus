package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-leb128"
	"github.com/polydawn/refmt/obj/atlas"
)

func init() {
	encoding.RegisterIpldCborType(attoFILAtlasEntry)
}

var attoPower = 18
var tenToTheEighteen = big.NewInt(10).Exp(big.NewInt(10), big.NewInt(18), nil)

// ZeroAttoFIL is the zero value for an AttoFIL, exported for consistency in construction of AttoFILs
var ZeroAttoFIL AttoFIL

var attoFILAtlasEntry = atlas.BuildEntry(AttoFIL{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(a AttoFIL) ([]byte, error) {
			return a.Bytes(), nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (AttoFIL, error) {
			return NewAttoFILFromBytes(x), nil
		})).
	Complete()

// UnmarshalJSON converts a byte array to an AttoFIL.
func (z *AttoFIL) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	af, ok := NewAttoFILFromFILString(s)
	if !ok {
		return errors.New("cannot convert string to token amount")
	}

	z.val = af.val

	return nil
}

// MarshalJSON converts an AttoFIL to a byte array and returns it.
func (z AttoFIL) MarshalJSON() ([]byte, error) {
	return json.Marshal(z.String())
}

// AttoFIL represents a signed multi-precision integer quantity of
// attofilecoin (atto is metric for 10**-18). The zero value for
// AttoFIL represents the value 0.
//
// Reasons for embedding a big.Int instead of *big.Int:
//   - We don't have check for nil in every method that does calculations.
//   - Serialization "symmetry" when serializing AttoFIL{}.
type AttoFIL struct{ val big.Int }

// NewAttoFIL allocates and returns a new AttoFIL set to x.
func NewAttoFIL(x *big.Int) AttoFIL {
	return AttoFIL{val: *big.NewInt(0).Set(x)}
}

// NewAttoFILFromFIL returns a new AttoFIL representing a quantity
// of attofilecoin equal to x filecoin.
func NewAttoFILFromFIL(x uint64) AttoFIL {
	xAsBigInt := big.NewInt(0).SetUint64(x)
	return NewAttoFIL(xAsBigInt.Mul(xAsBigInt, tenToTheEighteen))
}

// NewAttoFILFromBytes allocates and returns a new AttoFIL set
// to the value of buf as the bytes of a big-endian unsigned integer.
func NewAttoFILFromBytes(buf []byte) AttoFIL {
	return NewAttoFIL(leb128.ToBigInt(buf))
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
func NewAttoFILFromString(s string, base int) (out AttoFIL, err bool) {
	_, err = out.val.SetString(s, base)
	return
}

// Add sets z to the sum x+y and returns z.
func (z AttoFIL) Add(y AttoFIL) (out AttoFIL) {
	out.val.Add(&z.val, &y.val)
	return
}

// Sub sets z to the difference x-y and returns z.
func (z AttoFIL) Sub(y AttoFIL) (out AttoFIL) {
	out.val.Sub(&z.val, &y.val)
	return
}

// MulBigInt multiplies attoFIL by a given big int
func (z AttoFIL) MulBigInt(x *big.Int) (out AttoFIL) {
	out.val.Mul(&z.val, x)
	return
}

// DivCeil returns the minimum number of times this value can be divided into smaller amounts
// such that none of the smaller amounts are greater than the given divisor.
// Equal to ceil(z/y) if AttoFIL could be fractional.
// If y is zero a panic will occur.
func (z AttoFIL) DivCeil(y AttoFIL) AttoFIL {
	value, remainder := big.NewInt(0).DivMod(&z.val, &y.val, big.NewInt(0))

	if remainder.Cmp(big.NewInt(0)) == 0 {
		return NewAttoFIL(value)
	}

	return NewAttoFIL(big.NewInt(0).Add(value, big.NewInt(1)))
}

// Equal returns true if z = y
func (z AttoFIL) Equal(y AttoFIL) bool {
	return z.val.Cmp(&y.val) == 0
}

// LessThan returns true if z < y
func (z AttoFIL) LessThan(y AttoFIL) bool {
	return z.val.Cmp(&y.val) < 0
}

// GreaterThan returns true if z > y
func (z AttoFIL) GreaterThan(y AttoFIL) bool {
	return z.val.Cmp(&y.val) > 0
}

// LessEqual returns true if z <= y
func (z AttoFIL) LessEqual(y AttoFIL) bool {
	return z.val.Cmp(&y.val) <= 0
}

// GreaterEqual returns true if z >= y
func (z AttoFIL) GreaterEqual(y AttoFIL) bool {
	return z.val.Cmp(&y.val) >= 0
}

// IsPositive returns true if z is greater than zero.
func (z AttoFIL) IsPositive() bool {
	return z.GreaterThan(ZeroAttoFIL)
}

// IsNegative returns true if z is less than zero.
func (z AttoFIL) IsNegative() bool {
	return z.LessThan(ZeroAttoFIL)
}

// IsZero returns true if z equals zero.
func (z AttoFIL) IsZero() bool {
	return z.Equal(ZeroAttoFIL)
}

// AsBigInt returns the value as a big.Int
func (z AttoFIL) AsBigInt() (out *big.Int) {
	out = &big.Int{}
	out.Set(&z.val)
	return
}

// Bytes returns the absolute value of x as a big-endian byte slice.
func (z AttoFIL) Bytes() []byte {
	return leb128.FromBigInt(&z.val)
}

func (z AttoFIL) String() string {
	attoPadLength := strconv.Itoa(attoPower + 1)
	paddedStr := fmt.Sprintf("%0"+attoPadLength+"d", &z.val)
	decimaledStr := fmt.Sprintf("%s.%s", paddedStr[:len(paddedStr)-attoPower], paddedStr[len(paddedStr)-attoPower:])
	noTrailZeroStr := strings.TrimRight(decimaledStr, "0")
	return strings.TrimRight(noTrailZeroStr, ".")
}

// CalculatePrice treats z as a price in AttoFIL/Byte and applies it to numBytes to calculate a total price.
func (z AttoFIL) CalculatePrice(numBytes *BytesAmount) AttoFIL {
	ensureBytesAmounts(&numBytes)
	return z.MulBigInt(numBytes.val)
}
