package types

import (
	"encoding/json"
	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk/refmt/obj/atlas"
	"math/big"
)

func init() {
	cbor.RegisterCborType(bytesAmountAtlasEntry)
	ZeroBytes = NewBytesAmount(0)
}

// ZeroBytes represents a BytesAmount of 0
var ZeroBytes *BytesAmount

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
	var i big.Int
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*z = BytesAmount{val: &i}

	return nil
}

// MarshalJSON converts a BytesAmount to a byte array and returns it.
func (z BytesAmount) MarshalJSON() ([]byte, error) {
	return json.Marshal(z.val)
}

// An BytesAmount represents a signed multi-precision integer.
// The zero value for a BytesAmount represents the value 0.
type BytesAmount struct{ val *big.Int }

// NewBytesAmount allocates and returns a new BytesAmount set to x.
func NewBytesAmount(x int64) *BytesAmount {
	return &BytesAmount{val: big.NewInt(x)}
}

// NewBytesAmountFromBytes allocates and returns a new BytesAmount set
// to the value of buf as the bytes of a big-endian unsigned integer.
func NewBytesAmountFromBytes(buf []byte) *BytesAmount {
	ta := NewBytesAmount(0)
	ta.val.SetBytes(buf)
	return ta
}

// NewBytesAmountFromString allocates a new BytesAmount set to the value of s,
// interpreted in the given base, and returns it and a boolean indicating success.
func NewBytesAmountFromString(s string, base int) (*BytesAmount, bool) {
	ta := NewBytesAmount(0)
	val, ok := ta.val.SetString(s, base)
	ta.val = val // overkill
	return ta, ok
}

// Add sets z to the sum x+y and returns z.
func (z *BytesAmount) Add(y *BytesAmount) *BytesAmount {
	newVal := big.NewInt(0)
	newVal.Add(z.val, y.val)
	newZ := BytesAmount{val: newVal}
	return &newZ
}

// Sub sets z to the difference x-y and returns z.
func (z *BytesAmount) Sub(y *BytesAmount) *BytesAmount {
	newVal := big.NewInt(0)
	newVal.Sub(z.val, y.val)
	newZ := BytesAmount{val: newVal}
	return &newZ
}

// Equal returns true if z = y
func (z *BytesAmount) Equal(y *BytesAmount) bool {
	return z.val.Cmp(y.val) == 0
}

// LessThan returns true if z < y
func (z *BytesAmount) LessThan(y *BytesAmount) bool {
	return z.val.Cmp(y.val) < 0
}

// GreaterThan returns true if z > y
func (z *BytesAmount) GreaterThan(y *BytesAmount) bool {
	return z.val.Cmp(y.val) > 0
}

// LessEqual returns true if z <= y
func (z *BytesAmount) LessEqual(y *BytesAmount) bool {
	return z.val.Cmp(y.val) <= 0
}

// GreaterEqual returns true if z >= y
func (z *BytesAmount) GreaterEqual(y *BytesAmount) bool {
	return z.val.Cmp(y.val) >= 0
}

// IsPositive returns true if z is greater than zero.
func (z *BytesAmount) IsPositive() bool {
	return z.GreaterThan(ZeroBytes)
}

// IsNegative returns true if z is less than zero.
func (z *BytesAmount) IsNegative() bool {
	return z.LessThan(ZeroBytes)
}

// IsZero returns true if z equals zero.
func (z *BytesAmount) IsZero() bool {
	return z.Equal(ZeroBytes)
}

// Bytes returns the absolute value of x as a big-endian byte slice.
func (z *BytesAmount) Bytes() []byte {
	return z.val.Bytes()
}

func (z *BytesAmount) String() string {
	return z.val.String()
}

// Set sets z to x and returns z.
func (z *BytesAmount) Set(v *BytesAmount) *BytesAmount {
	z.val.Set(v.val)
	return z
}
