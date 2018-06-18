package types

import (
	"encoding/json"
	"errors"
	"math/big"

	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"
	"gx/ipfs/QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk/refmt/obj/atlas"
)

// NOTE -- ALL *TokenAmount methods must call ensureBytesAmounts with refs to every user-supplied value before use.

func init() {
	cbor.RegisterCborType(tokenAmountAtlasEntry)
	ZeroToken = NewTokenAmount(0)
}

// ZeroToken represents a TokenAmount of 0
var ZeroToken *TokenAmount // Singular rather than plural, on the assumption that we're using Token as a mass noun.

// ensureTokenAmounts takes a variable number of refs -- variables holding *TokenAmount -- and sets their values
// to the ZeroToken (the zero value for the type) if their values are nil.
func ensureTokenAmounts(refs ...**TokenAmount) {
	for _, ref := range refs {
		if *ref == nil {
			*ref = ZeroToken
		}
	}
}

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
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	token, ok := NewTokenAmountFromString(s, 10)
	if !ok {
		return errors.New("cannot convert string to token amount")
	}

	*z = *token

	return nil
}

// MarshalJSON converts a TokenAmount to a byte array and returns it.
func (z TokenAmount) MarshalJSON() ([]byte, error) {
	return json.Marshal(z.val.String())
}

// An TokenAmount represents a signed multi-precision integer.
// The zero value for a TokenAmount represents the value 0.
type TokenAmount struct{ val *big.Int }

// NewTokenAmount allocates and returns a new TokenAmount set to x.
func NewTokenAmount(x uint64) *TokenAmount {
	return &TokenAmount{val: big.NewInt(0).SetUint64(x)}
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
	ensureTokenAmounts(&z, &y)
	newVal := big.NewInt(0)
	newVal.Add(z.val, y.val)
	newZ := TokenAmount{val: newVal}
	return &newZ
}

// Sub sets z to the difference x-y and returns z.
func (z *TokenAmount) Sub(y *TokenAmount) *TokenAmount {
	ensureTokenAmounts(&z, &y)
	newVal := big.NewInt(0)
	newVal.Sub(z.val, y.val)
	newZ := TokenAmount{val: newVal}
	return &newZ
}

// Equal returns true if z = y
func (z *TokenAmount) Equal(y *TokenAmount) bool {
	ensureTokenAmounts(&z, &y)
	return z.val.Cmp(y.val) == 0
}

// LessThan returns true if z < y
func (z *TokenAmount) LessThan(y *TokenAmount) bool {
	ensureTokenAmounts(&z, &y)
	return z.val.Cmp(y.val) < 0
}

// GreaterThan returns true if z > y
func (z *TokenAmount) GreaterThan(y *TokenAmount) bool {
	ensureTokenAmounts(&z, &y)
	return z.val.Cmp(y.val) > 0
}

// LessEqual returns true if z <= y
func (z *TokenAmount) LessEqual(y *TokenAmount) bool {
	ensureTokenAmounts(&z, &y)
	return z.val.Cmp(y.val) <= 0
}

// GreaterEqual returns true if z >= y
func (z *TokenAmount) GreaterEqual(y *TokenAmount) bool {
	ensureTokenAmounts(&z, &y)
	return z.val.Cmp(y.val) >= 0
}

// IsPositive returns true if z is greater than zero.
func (z *TokenAmount) IsPositive() bool {
	ensureTokenAmounts(&z)
	return z.GreaterThan(ZeroToken)
}

// IsNegative returns true if z is less than zero.
func (z *TokenAmount) IsNegative() bool {
	ensureTokenAmounts(&z)
	return z.LessThan(ZeroToken)
}

// IsZero returns true if z equals zero.
func (z *TokenAmount) IsZero() bool {
	ensureTokenAmounts(&z)
	return z.Equal(ZeroToken)
}

// Bytes returns the absolute value of x as a big-endian byte slice.
func (z *TokenAmount) Bytes() []byte {
	ensureTokenAmounts(&z)
	return z.val.Bytes()
}

func (z *TokenAmount) String() string {
	ensureTokenAmounts(&z)
	return z.val.String()
}

// CalculatePrice treats z as a price in Tokens/Byte and applies it to numBytes to calculate a total price.
func (z *TokenAmount) CalculatePrice(numBytes *BytesAmount) *TokenAmount {
	ensureTokenAmounts(&z)
	ensureBytesAmounts(&numBytes)
	unitPrice := z

	newVal := big.NewInt(0)
	newVal.Mul(unitPrice.val, numBytes.val)

	return &TokenAmount{val: newVal}
}
