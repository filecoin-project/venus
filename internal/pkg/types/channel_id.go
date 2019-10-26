package types

import (
	"encoding/json"
	"math/big"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-leb128"
	"github.com/polydawn/refmt/obj/atlas"
)

func init() {
	encoding.RegisterIpldCborType(channelIDAtlasEntry)
}

var channelIDAtlasEntry = atlas.BuildEntry(ChannelID{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(i ChannelID) ([]byte, error) {
			return i.Bytes(), nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (ChannelID, error) {
			return *NewChannelIDFromBytes(x), nil
		})).
	Complete()

// UnmarshalJSON converts a byte array to a ChannelID.
func (z *ChannelID) UnmarshalJSON(b []byte) error {
	var i big.Int
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*z = ChannelID{val: &i}

	return nil
}

// MarshalJSON converts a ChannelID to a byte array and returns it.
func (z ChannelID) MarshalJSON() ([]byte, error) {
	return json.Marshal(z.val)
}

// An ChannelID is a signed multi-precision integer.
type ChannelID struct{ val *big.Int }

// NewChannelID allocates and returns a new ChannelID set to x.
func NewChannelID(x uint64) *ChannelID {
	return &ChannelID{val: big.NewInt(0).SetUint64(x)}
}

// NewChannelIDFromBytes allocates and returns a new ChannelID set
// to the value of buf as the bytes of a big-endian unsigned integer.
func NewChannelIDFromBytes(buf []byte) *ChannelID {
	ci := NewChannelID(0)
	ci.val = leb128.ToBigInt(buf)
	return ci
}

// NewChannelIDFromString allocates a new ChannelID set to the value of s,
// interpreted in the given base, and returns it and a boolean indicating success.
func NewChannelIDFromString(s string, base int) (*ChannelID, bool) {
	ta := NewChannelID(0)
	val, ok := ta.val.SetString(s, base)
	ta.val = val // overkill
	return ta, ok
}

// Bytes returns the absolute value of x as a big-endian byte slice.
func (z *ChannelID) Bytes() []byte {
	return leb128.FromBigInt(z.val)
}

// Equal returns true if z = y
func (z *ChannelID) Equal(y *ChannelID) bool {
	return z.val.Cmp(y.val) == 0
}

// String returns a string version of the ID
func (z *ChannelID) String() string {
	return z.val.String()
}

// KeyString returns a compact string version of the ID
func (z *ChannelID) KeyString() string {
	return z.val.String()
}

// Inc increments the value of the channel id
func (z *ChannelID) Inc() *ChannelID {
	return NewChannelID(z.val.Uint64() + 1)
}
