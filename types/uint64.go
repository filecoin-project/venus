package types

import (
	"strconv"
	"strings"

	"github.com/filecoin-project/go-leb128"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/polydawn/refmt/obj/atlas"
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
