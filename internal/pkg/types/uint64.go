package types

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-leb128"
	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
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

// BigToUint64 converts a big Int to a uint64.  It will error if
// the Int is too big to fit into 64 bits or is negative
func BigToUint64(bi fbig.Int) (uint64, error) {
	if !bi.Int.IsUint64() {
		return 0, fmt.Errorf("Int: %s could not be represented as uint64", bi.String())
	}
	return bi.Uint64(), nil
}

// Uint64ToBig converts a uint64 to a big Int.  Precodition: don't overflow int64.
func Uint64ToBig(u uint64) fbig.Int {
	return fbig.NewInt(int64(u))
}
