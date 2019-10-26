package types

import (
	"strconv"
	"strings"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-leb128"
	"github.com/polydawn/refmt/obj/atlas"
)

func init() {
	encoding.RegisterIpldCborType(uint64AtlasEntry)
}

var uint64AtlasEntry = atlas.BuildEntry(Uint64(0)).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(u Uint64) ([]byte, error) {
			return encoding.Encode(u)
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (Uint64, error) {
			var aux Uint64
			if err := encoding.Decode(x, &aux); err != nil {
				return aux, err
			}
			return aux, nil
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

// Encode encodes the object on an encoder.
func (u Uint64) Encode(encoder encoding.Encoder) error {
	return encoder.EncodeArray(leb128.FromUInt64(uint64(u)))
}

// Decode decodes the object from an decoder.
func (u *Uint64) Decode(decoder encoding.Decoder) error {
	b := make([]byte, 8)
	if err := decoder.DecodeArray(&b); err != nil {
		return err
	}
	*u = Uint64(leb128.ToUInt64(b))
	return nil
}
