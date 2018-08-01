package types

import (
	"encoding/json"
	"math/big"

	"gx/ipfs/QmSKyB5faguXT4NqbrXpnRXqaVj5DhSm7x9BtzFydBY1UK/go-leb128"
	cbor "gx/ipfs/QmSyK1ZiAP98YvnxsTfQpb669V2xeTHRbG4Y6fgKS3vVSd/go-ipld-cbor"
	"gx/ipfs/QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk/refmt/obj/atlas"
)

func init() {
	cbor.RegisterCborType(blockHeightAtlasEntry)
}

var blockHeightAtlasEntry = atlas.BuildEntry(BlockHeight{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(i BlockHeight) ([]byte, error) {
			return i.Bytes(), nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (BlockHeight, error) {
			return *NewBlockHeightFromBytes(x), nil
		})).
	Complete()

// UnmarshalJSON converts a byte array to a BlockHeight.
func (z *BlockHeight) UnmarshalJSON(b []byte) error {
	var i big.Int
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*z = BlockHeight{val: &i}

	return nil
}

// MarshalJSON converts a BlockHeight to a byte array and returns it.
func (z BlockHeight) MarshalJSON() ([]byte, error) {
	return json.Marshal(z.val)
}

// An BlockHeight is a signed multi-precision integer.
type BlockHeight struct{ val *big.Int }

// NewBlockHeight allocates and returns a new BlockHeight set to x.
func NewBlockHeight(x uint64) *BlockHeight {
	return &BlockHeight{val: big.NewInt(0).SetUint64(x)}
}

// NewBlockHeightFromBytes allocates and returns a new BlockHeight set
// to the value of buf as the bytes of a big-endian unsigned integer.
func NewBlockHeightFromBytes(buf []byte) *BlockHeight {
	bh := NewBlockHeight(0)
	bh.val = leb128.ToBigInt(buf)
	return bh
}

// NewBlockHeightFromString allocates a new BlockHeight set to the value of s,
// interpreted in the given base, and returns it and a boolean indicating success.
func NewBlockHeightFromString(s string, base int) (*BlockHeight, bool) {
	bh := NewBlockHeight(0)
	val, ok := bh.val.SetString(s, base)
	bh.val = val // overkill
	return bh, ok
}

// Bytes returns the absolute value of x as a big-endian byte slice.
func (z *BlockHeight) Bytes() []byte {
	return leb128.FromBigInt(z.val)
}

// Equal returns true if z = y
func (z *BlockHeight) Equal(y *BlockHeight) bool {
	return z.val.Cmp(y.val) == 0
}

// String returns a string version of the ID
func (z *BlockHeight) String() string {
	return z.val.String()
}

// LessThan returns true if z < y
func (z *BlockHeight) LessThan(y *BlockHeight) bool {
	return z.val.Cmp(y.val) < 0
}

// GreaterThan returns true if z > y
func (z *BlockHeight) GreaterThan(y *BlockHeight) bool {
	return z.val.Cmp(y.val) > 0
}

// LessEqual returns true if z <= y
func (z *BlockHeight) LessEqual(y *BlockHeight) bool {
	return z.val.Cmp(y.val) <= 0
}

// GreaterEqual returns true if z >= y
func (z *BlockHeight) GreaterEqual(y *BlockHeight) bool {
	return z.val.Cmp(y.val) >= 0
}
