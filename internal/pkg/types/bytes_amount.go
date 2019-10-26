package types

import (
	"encoding/json"
	"math/big"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-leb128"
	"github.com/pkg/errors"
	"github.com/polydawn/refmt/obj/atlas"
)

// NOTE -- All *BytesAmount methods must call ensureBytesAmounts with refs to every user-supplied value before use.

func init() {
	encoding.RegisterIpldCborType(bytesAmountAtlasEntry)
	ZeroBytes = NewBytesAmount(0)
}

// ZeroBytes represents a BytesAmount of 0
var ZeroBytes *BytesAmount

// ensureBytesAmounts takes a variable number of refs -- variables holding *BytesAmount -- and sets their values
// to ZeroBytes (the zero value for the type) if their values are nil.
func ensureBytesAmounts(refs ...**BytesAmount) {
	for _, ref := range refs {
		if *ref == nil {
			*ref = ZeroBytes
		}
	}
}

var bytesAmountAtlasEntry = atlas.BuildEntry(BytesAmount{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(i BytesAmount) ([]byte, error) {
			return i.Bytes(), nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (BytesAmount, error) {
			return *NewBytesAmountFromBytes(x), nil
		})).
	Complete()

// UnmarshalJSON converts a byte array to a BytesAmount.
func (z *BytesAmount) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	bytes, ok := NewBytesAmountFromString(s, 10)
	if !ok {
		return errors.New("Cannot convert string to bytes amount")
	}

	*z = *bytes

	return nil
}

// MarshalJSON converts a BytesAmount to a byte array and returns it.
func (z BytesAmount) MarshalJSON() ([]byte, error) {
	return json.Marshal(z.val.String())
}

// An BytesAmount represents a signed multi-precision integer.
// The zero value for a BytesAmount represents the value 0.
type BytesAmount struct{ val *big.Int }

// NewBytesAmount allocates and returns a new BytesAmount set to x.
func NewBytesAmount(x uint64) *BytesAmount {
	return &BytesAmount{val: big.NewInt(0).SetUint64(x)}
}

// NewBytesAmountFromBytes allocates and returns a new BytesAmount set
// to the value of buf as the bytes of a big-endian unsigned integer.
func NewBytesAmountFromBytes(buf []byte) *BytesAmount {
	ba := NewBytesAmount(0)
	ba.val = leb128.ToBigInt(buf)
	return ba
}

// NewBytesAmountFromString allocates a new BytesAmount set to the value of s,
// interpreted in the given base, and returns it and a boolean indicating success.
func NewBytesAmountFromString(s string, base int) (*BytesAmount, bool) {
	ba := NewBytesAmount(0)
	val, ok := ba.val.SetString(s, base)
	ba.val = val // overkill
	return ba, ok
}

// Add returns the sum z+y.
func (z *BytesAmount) Add(y *BytesAmount) *BytesAmount {
	ensureBytesAmounts(&z, &y)
	newVal := big.NewInt(0)
	newVal.Add(z.val, y.val)
	newZ := BytesAmount{val: newVal}
	return &newZ
}

// Sub returns the difference z-y.
func (z *BytesAmount) Sub(y *BytesAmount) *BytesAmount {
	ensureBytesAmounts(&z, &y)
	newVal := big.NewInt(0)
	newVal.Sub(z.val, y.val)
	newZ := BytesAmount{val: newVal}
	return &newZ
}

// Mul returns the product z*y.
func (z *BytesAmount) Mul(y *BytesAmount) *BytesAmount {
	ensureBytesAmounts(&z, &y)
	newVal := big.NewInt(0)
	newVal.Mul(z.val, y.val)
	newZ := BytesAmount{val: newVal}
	return &newZ
}

// Quo returns the quotient z/y for y != 0. If y == 0, a division-by-zero
// run-time panic occurs.
func (z *BytesAmount) Quo(y *BytesAmount) *BytesAmount {
	ensureBytesAmounts(&z, &y)
	newVal := big.NewInt(0)
	newVal.Quo(z.val, y.val)
	newZ := BytesAmount{val: newVal}
	return &newZ
}

// Equal returns true if z = y.
func (z *BytesAmount) Equal(y *BytesAmount) bool {
	ensureBytesAmounts(&z, &y)
	return z.val.Cmp(y.val) == 0
}

// LessThan returns true if z < y.
func (z *BytesAmount) LessThan(y *BytesAmount) bool {
	ensureBytesAmounts(&z, &y)
	return z.val.Cmp(y.val) < 0
}

// GreaterThan returns true if z > y.
func (z *BytesAmount) GreaterThan(y *BytesAmount) bool {
	ensureBytesAmounts(&z, &y)
	return z.val.Cmp(y.val) > 0
}

// LessEqual returns true if z <= y.
func (z *BytesAmount) LessEqual(y *BytesAmount) bool {
	ensureBytesAmounts(&z, &y)
	return z.val.Cmp(y.val) <= 0
}

// GreaterEqual returns true if z >= y.
func (z *BytesAmount) GreaterEqual(y *BytesAmount) bool {
	ensureBytesAmounts(&z, &y)
	return z.val.Cmp(y.val) >= 0
}

// IsPositive returns true if z is greater than zero.
func (z *BytesAmount) IsPositive() bool {
	ensureBytesAmounts(&z)
	return z.GreaterThan(ZeroBytes)
}

// IsNegative returns true if z is less than zero.
func (z *BytesAmount) IsNegative() bool {
	ensureBytesAmounts(&z)
	return z.LessThan(ZeroBytes)
}

// IsZero returns true if z equals zero.
func (z *BytesAmount) IsZero() bool {
	ensureBytesAmounts(&z)
	return z.Equal(ZeroBytes)
}

// Bytes returns the absolute value of z as a big-endian byte slice.
func (z *BytesAmount) Bytes() []byte {
	ensureBytesAmounts(&z)
	return leb128.FromBigInt(z.val)
}

func (z *BytesAmount) String() string {
	ensureBytesAmounts(&z)
	return z.val.String()
}

// Uint64 returns the uint64 representation of z. If z cannot be represented as
// a uint64, the result is undefined.
func (z *BytesAmount) Uint64() uint64 {
	ensureBytesAmounts(&z)
	return z.val.Uint64()
}

// BigInt returns the big.Int representation of z.
func (z *BytesAmount) BigInt() *big.Int {
	ensureBytesAmounts(&z)
	return (&big.Int{}).Set(z.val)
}
