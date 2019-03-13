package types

import (
	"strconv"
	"strings"

	"gx/ipfs/QmSKyB5faguXT4NqbrXpnRXqaVj5DhSm7x9BtzFydBY1UK/go-leb128"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"
	"gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt/obj/atlas"
)

func init() {
	cbor.RegisterCborType(uint64AtlasEntry)
}

var uint64AtlasEntry = atlas.BuildEntry(Uint64(0)).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(u Uint64) ([]byte, error) {
			return leb128.FromUInt64(uint64(u)), nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (Uint64, error) {
			return Uint64(leb128.ToUInt64(x)), nil
		})).
	Complete()

// Uint64 is an unsigned 64-bit variable-length-encoded integer.
type Uint64 uint64

// MarshalJSON converts a Uint64 to a json string and returns it.
func (u Uint64) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatUint(uint64(u), 10) + `"`), nil
}

// UnmarshalJSON converts a json string to a Uint64.
func (u *Uint64) UnmarshalJSON(b []byte) error {
	val, err := strconv.ParseUint(strings.Trim(string(b), `"`), 10, 64)
	if err != nil {
		return err
	}

	*u = Uint64(val)
	return nil
}
