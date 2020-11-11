package enccid

import (
	"encoding/json"
	"fmt"

	cbor "github.com/fxamacker/cbor/v2"
	cid "github.com/ipfs/go-cid"
	ipldcbor "github.com/ipfs/go-ipld-cbor"
)

// Cid is a cid wrapper that implements UnmarshalCBOR and MarshalCBOR.
// From ipld-cbor's perspective it is technically a pointer to a cid, because
// it maps `cbor null-val` <==> `cid.Undef`
type Cid struct {
	cid.Cid
}

// Undef wraps cid.Undef
var Undef = NewCid(cid.Undef)

// NewCid creates an Cid struct from a cid
func NewCid(c cid.Cid) Cid {
	return Cid{c}
}

// MarshalCBOR converts the wrapped cid to bytes
func (w Cid) MarshalCBOR() ([]byte, error) {
	// handle undef cid by writing null
	// TODO: remove this handling after removing paths that attempt to encode an Undef CID
	// This should never appear on chain, and the only usages are tests.
	// https://github.com/filecoin-project/venus/issues/3931
	if w.Equals(cid.Undef) {
		return []byte{0xf6}, nil
	}

	// tag = 42
	tag0 := byte(0xd8)
	tag1 := byte(0x2a)

	raw, err := castCidToBytes(w.Cid)
	if err != nil {
		return nil, err
	}
	// because we need to do the cbor tag outside the byte string we are forced
	// to write the cbor type-len value for a byte string of raw's length
	cborLen, err := cbor.Marshal(len(raw))
	if err != nil {
		return nil, err
	}
	cborLen[0] |= 0x40 // flip major type 0 to major type 2
	prefixLen := len(cborLen) + 2

	result := make([]byte, len(cborLen)+len(raw)+2, len(cborLen)+len(raw)+2)
	result[0] = tag0
	result[1] = tag1
	copy(result[2:prefixLen], cborLen)
	copy(result[prefixLen:], raw)

	return result, nil
}

// UnmarshalCBOR fills the wrapped cid according to the cbor encoded bytes
func (w *Cid) UnmarshalCBOR(cborBs []byte) error {
	if len(cborBs) == 0 {
		return fmt.Errorf("nil bytes does not decode to cid")
	}
	// check undef cid
	if len(cborBs) == 1 {
		if cborBs[0] != 0xf6 {
			return fmt.Errorf("invalid cbor bytes: %x for cid", cborBs[0])
		}
		// this is a pointer to an undefined cid
		w.Cid = cid.Undef
		return nil
	}

	// check tag:
	if cborBs[0] != 0xd8 || cborBs[1] != 0x2a {
		return fmt.Errorf("ipld cbor tags cids with tag 42 not %x", cborBs[:2])
	}
	cborBs = cborBs[2:]
	// strip len:
	var cidBs []byte
	err := cbor.Unmarshal(cborBs, &cidBs)
	if err != nil {
		return err
	}

	w.Cid, err = castBytesToCid(cidBs)
	return err
}

// UnmarshalJSON defers to cid json unmarshalling
func (w *Cid) UnmarshalJSON(jsonBs []byte) error {
	return json.Unmarshal(jsonBs, &w.Cid)
}

// MarshalJSON defers to cid json marshalling
func (w Cid) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Cid)
}

// This is lifted from go-ipld-cbor but should probably be exported from there.
func castBytesToCid(x []byte) (cid.Cid, error) {
	if len(x) == 0 {
		return cid.Cid{}, ipldcbor.ErrEmptyLink
	}

	if x[0] != 0 {
		return cid.Cid{}, ipldcbor.ErrInvalidMultibase
	}

	c, err := cid.Cast(x[1:])
	if err != nil {
		return cid.Cid{}, ipldcbor.ErrInvalidLink
	}

	return c, nil
}

// This is lifted from go-ipld-cbor but should probably be exported from there.
func castCidToBytes(link cid.Cid) ([]byte, error) {
	if !link.Defined() {
		return nil, ipldcbor.ErrEmptyLink
	}
	return append([]byte{0}, link.Bytes()...), nil
}

func EncidToCidArr(idArr []Cid) []cid.Cid {
	result := make([]cid.Cid, len(idArr))
	for index, val := range idArr {
		result[index] = val.Cid
	}
	return result
}

func WrapCid(ids []cid.Cid) []Cid {
	result := make([]Cid, len(ids))
	for index, id := range ids {
		result[index] = NewCid(id)
	}
	return result
}
