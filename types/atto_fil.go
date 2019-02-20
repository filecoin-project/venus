package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"gx/ipfs/QmSKyB5faguXT4NqbrXpnRXqaVj5DhSm7x9BtzFydBY1UK/go-leb128"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"
	"gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt/obj/atlas"
)

// NOTE -- ALL *AttoFIL methods must call ensureZeroAmounts with refs to every user-supplied value before use.

func init() {
	cbor.RegisterCborType(attoFILAtlasEntry)
	ZeroAttoFIL = NewZeroAttoFIL()
}

var attoPower = 18
var tenToTheEighteen = big.NewInt(10).Exp(big.NewInt(10), big.NewInt(18), nil)

// ZeroAttoFIL represents an AttoFIL quantity of 0
var ZeroAttoFIL *AttoFIL

// ensureZeroAmounts takes a variable number of refs -- variables holding *AttoFIL --
// and sets their values to the ZeroAttoFIL (the zero value for the type) if their values are nil.
func ensureZeroAmounts(refs ...**AttoFIL) {
	for _, ref := range refs {
		if *ref == nil {
			*ref = ZeroAttoFIL
		}
	}
}

var attoFILAtlasEntry = atlas.BuildEntry(AttoFIL{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(a AttoFIL) ([]byte, error) {
			return a.Bytes(), nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (AttoFIL, error) {
			return *NewAttoFILFromBytes(x), nil
		})).
	Complete()

// UnmarshalJSON converts a byte array to an AttoFIL.
func (z *AttoFIL) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	token, ok := NewAttoFILFromFILString(s)
	if !ok {
		return errors.New("cannot convert string to token amount")
	}

	*z = *token

	return nil
}

// MarshalJSON converts an AttoFIL to a byte array and returns it.
func (z AttoFIL) MarshalJSON() ([]byte, error) {
	return json.Marshal(z.String())
}

// AttoFIL represents a signed multi-precision integer quantity of
// attofilecoin (atto is metric for 10**-18). The zero value for
// AttoFIL represents the value 0.
type AttoFIL struct{ val *big.Int }

// NewAttoFIL allocates and returns a new AttoFIL set to x.
func NewAttoFIL(x *big.Int) *AttoFIL {
	return &AttoFIL{val: big.NewInt(0).Set(x)}
}

// NewZeroAttoFIL returns a new zero quantity of attofilecoin. It is
// different from ZeroAttoFIL in that this value may be used/mutated.
func NewZeroAttoFIL() *AttoFIL {
	return NewAttoFIL(big.NewInt(0))
}

// NewAttoFILFromFIL returns a new AttoFIL representing a quantity
// of attofilecoin equal to x filecoin.
func NewAttoFILFromFIL(x uint64) *AttoFIL {
	xAsBigInt := big.NewInt(0).SetUint64(x)
	return NewAttoFIL(xAsBigInt.Mul(xAsBigInt, tenToTheEighteen))
}

// NewAttoFILFromBytes allocates and returns a new AttoFIL set
// to the value of buf as the bytes of a big-endian unsigned integer.
func NewAttoFILFromBytes(buf []byte) *AttoFIL {
	af := NewZeroAttoFIL()
	af.val = leb128.ToBigInt(buf)
	return af
}

// NewAttoFILFromFILString allocates a new AttoFIL set to the value of s filecoin,
// interpreted as a decimal in base 10, and returns it and a boolean indicating success.
func NewAttoFILFromFILString(s string) (*AttoFIL, bool) {
	splitNumber := strings.Split(s, ".")
	// If '.' is absent from string, add an empty string to become the decimal part
	if len(splitNumber) == 1 {
		splitNumber = append(splitNumber, "")
	}
	intPart := splitNumber[0]
	decPart := splitNumber[1]
	// A decimal part longer than 18 digits should be an error
	if len(decPart) > attoPower || len(splitNumber) > 2 {
		return nil, false
	}
	// The decimal is right padded with 0's if it less than 18 digits long
	for len(decPart) < attoPower {
		decPart += "0"
	}

	return NewAttoFILFromString(intPart+decPart, 10)
}

// NewAttoFILFromString allocates a new AttoFIL set to the value of s attofilecoin,
// interpreted in the given base, and returns it and a boolean indicating success.
func NewAttoFILFromString(s string, base int) (*AttoFIL, bool) {
	af := NewZeroAttoFIL()
	_, ok := af.val.SetString(s, base)
	return af, ok
}

// Add sets z to the sum x+y and returns z.
func (z *AttoFIL) Add(y *AttoFIL) *AttoFIL {
	ensureZeroAmounts(&z, &y)
	newVal := big.NewInt(0)
	newVal.Add(z.val, y.val)
	newZ := AttoFIL{val: newVal}
	return &newZ
}

// Sub sets z to the difference x-y and returns z.
func (z *AttoFIL) Sub(y *AttoFIL) *AttoFIL {
	ensureZeroAmounts(&z, &y)
	newVal := big.NewInt(0)
	newVal.Sub(z.val, y.val)
	newZ := AttoFIL{val: newVal}
	return &newZ
}

// MulBigInt multiplies attoFIL by a given big int
func (z *AttoFIL) MulBigInt(x *big.Int) *AttoFIL {
	newVal := big.NewInt(0)
	newVal.Mul(z.val, x)
	return &AttoFIL{val: newVal}
}

// DivCeil returns the minimum number of times this value can be divided into smaller amounts
// such that none of the smaller amounts are greater than the given divisor.
// Equal to ceil(z/y) if AttoFIL could be fractional.
// If y is zero a panic will occur.
func (z *AttoFIL) DivCeil(y *AttoFIL) *AttoFIL {
	value, remainder := big.NewInt(0).DivMod(z.val, y.val, big.NewInt(0))

	if remainder.Cmp(big.NewInt(0)) == 0 {
		return NewAttoFIL(value)
	}

	return NewAttoFIL(big.NewInt(0).Add(value, big.NewInt(1)))
}

// Equal returns true if z = y
func (z *AttoFIL) Equal(y *AttoFIL) bool {
	ensureZeroAmounts(&z, &y)
	return z.val.Cmp(y.val) == 0
}

// LessThan returns true if z < y
func (z *AttoFIL) LessThan(y *AttoFIL) bool {
	ensureZeroAmounts(&z, &y)
	return z.val.Cmp(y.val) < 0
}

// GreaterThan returns true if z > y
func (z *AttoFIL) GreaterThan(y *AttoFIL) bool {
	ensureZeroAmounts(&z, &y)
	return z.val.Cmp(y.val) > 0
}

// LessEqual returns true if z <= y
func (z *AttoFIL) LessEqual(y *AttoFIL) bool {
	ensureZeroAmounts(&z, &y)
	return z.val.Cmp(y.val) <= 0
}

// GreaterEqual returns true if z >= y
func (z *AttoFIL) GreaterEqual(y *AttoFIL) bool {
	ensureZeroAmounts(&z, &y)
	return z.val.Cmp(y.val) >= 0
}

// IsPositive returns true if z is greater than zero.
func (z *AttoFIL) IsPositive() bool {
	ensureZeroAmounts(&z)
	return z.GreaterThan(ZeroAttoFIL)
}

// IsNegative returns true if z is less than zero.
func (z *AttoFIL) IsNegative() bool {
	ensureZeroAmounts(&z)
	return z.LessThan(ZeroAttoFIL)
}

// IsZero returns true if z equals zero.
func (z *AttoFIL) IsZero() bool {
	ensureZeroAmounts(&z)
	return z.Equal(ZeroAttoFIL)
}

// Bytes returns the absolute value of x as a big-endian byte slice.
func (z *AttoFIL) Bytes() []byte {
	ensureZeroAmounts(&z)
	return leb128.FromBigInt(z.val)
}

func (z *AttoFIL) String() string {
	ensureZeroAmounts(&z)
	attoPadLength := strconv.Itoa(attoPower + 1)
	paddedStr := fmt.Sprintf("%0"+attoPadLength+"d", z.val)
	decimaledStr := fmt.Sprintf("%s.%s", paddedStr[:len(paddedStr)-attoPower], paddedStr[len(paddedStr)-attoPower:])
	noTrailZeroStr := strings.TrimRight(decimaledStr, "0")
	return strings.TrimRight(noTrailZeroStr, ".")
}

// CalculatePrice treats z as a price in AttoFIL/Byte and applies it to numBytes to calculate a total price.
func (z *AttoFIL) CalculatePrice(numBytes *BytesAmount) *AttoFIL {
	ensureZeroAmounts(&z)
	ensureBytesAmounts(&numBytes)
	unitPrice := z

	newVal := big.NewInt(0)
	newVal.Mul(unitPrice.val, numBytes.val)

	return &AttoFIL{val: newVal}
}
