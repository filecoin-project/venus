package types

import (
	"encoding/json"
	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk/refmt/obj/atlas"
	"math/big"
)

func init() {
	cbor.RegisterCborType(tokenAmountAtlasEntry)
	ZeroToken = NewTokenAmount(0)
}

// ZeroToken represents a TokenAmount of 0
var ZeroToken *TokenAmount // Singular rather than plural, on the assumption that we're using Token as a mass noun.

var tokenAmountAtlasEntry = atlas.BuildEntry(TokenAmount{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(i TokenAmount) ([]byte, error) {
			return i.Bytes(), nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (TokenAmount, error) {
			return *NewTokenAmountFromBytes(x), nil
		})).
	Complete()

// UnmarshalJSON converts a byte array to a TokenAmount.
func (z *TokenAmount) UnmarshalJSON(b []byte) error {
	var i big.Int
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*z = TokenAmount{val: &i}

	return nil
}

// MarshalJSON converts a TokenAmount to a byte array and returns it.
func (z TokenAmount) MarshalJSON() ([]byte, error) {
	return json.Marshal(z.val)
}

// An TokenAmount represents a signed multi-precision integer.
// The zero value for a TokenAmount represents the value 0.
type TokenAmount struct{ val *big.Int }

// NewTokenAmount allocates and returns a new TokenAmount set to x.
func NewTokenAmount(x int64) *TokenAmount {
	return &TokenAmount{val: big.NewInt(x)}
}

// NewTokenAmountFromBytes allocates and returns a new TokenAmount set
// to the value of buf as the bytes of a big-endian unsigned integer.
func NewTokenAmountFromBytes(buf []byte) *TokenAmount {
	ta := NewTokenAmount(0)
	ta.val.SetBytes(buf)
	return ta
}

// NewTokenAmountFromString allocates a new TokenAmount set to the value of s,
// interpreted in the given base, and returns it and a boolean indicating success.
func NewTokenAmountFromString(s string, base int) (*TokenAmount, bool) {
	ta := NewTokenAmount(0)
	val, ok := ta.val.SetString(s, base)
	ta.val = val // overkill
	return ta, ok
}

// Add sets z to the sum x+y and returns z.
func (z *TokenAmount) Add(y *TokenAmount) *TokenAmount {
	newVal := big.NewInt(0)
	newVal.Add(z.val, y.val)
	newZ := TokenAmount{val: newVal}
	return &newZ
}

// Sub sets z to the difference x-y and returns z.
func (z *TokenAmount) Sub(y *TokenAmount) *TokenAmount {
	newVal := big.NewInt(0)
	newVal.Sub(z.val, y.val)
	newZ := TokenAmount{val: newVal}
	return &newZ
}

// Equal returns true if z = y
func (z *TokenAmount) Equal(y *TokenAmount) bool {
	return z.val.Cmp(y.val) == 0
}

// LessThan returns true if z < y
func (z *TokenAmount) LessThan(y *TokenAmount) bool {
	return z.val.Cmp(y.val) < 0
}

// GreaterThan returns true if z > y
func (z *TokenAmount) GreaterThan(y *TokenAmount) bool {
	return z.val.Cmp(y.val) > 0
}

// LessEqual returns true if z <= y
func (z *TokenAmount) LessEqual(y *TokenAmount) bool {
	return z.val.Cmp(y.val) <= 0
}

// GreaterEqual returns true if z >= y
func (z *TokenAmount) GreaterEqual(y *TokenAmount) bool {
	return z.val.Cmp(y.val) >= 0
}

// IsPositive returns true if z is greater than zero.
func (z *TokenAmount) IsPositive() bool {
	return z.GreaterThan(ZeroToken)
}

// IsNegative returns true if z is less than zero.
func (z *TokenAmount) IsNegative() bool {
	return z.LessThan(ZeroToken)
}

// IsZero returns true if z equals zero.
func (z *TokenAmount) IsZero() bool {
	return z.Equal(ZeroToken)
}

// Bytes returns the absolute value of x as a big-endian byte slice.
func (z *TokenAmount) Bytes() []byte {
	return z.val.Bytes()
}

func (z *TokenAmount) String() string {
	return z.val.String()
}

// Set sets z to x and returns z.
func (z *TokenAmount) Set(v *TokenAmount) *TokenAmount {
	z.val.Set(v.val)
	return z
}

// CalculatePrice treats z as a price in Tokens/Byte and applies it to numBytes to calculate a total price.
func (z *TokenAmount) CalculatePrice(numBytes *BytesAmount) *TokenAmount {
	unitPrice := z
	total := NewTokenAmount(0)
	total.val.Mul(unitPrice.val, numBytes.val)
	return total
}
