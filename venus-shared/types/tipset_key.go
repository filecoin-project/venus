package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/filecoin-project/go-state-types/abi"
	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	cbg "github.com/whyrusleeping/cbor-gen"
)

var log = logging.Logger("types")

// TipSetKey is an immutable set of CIDs forming a unique key for a TipSet.
// Equal keys will have equivalent iteration order. CIDs are maintained in
// the same order as the canonical iteration order of blocks in a tipset (which is by ticket).
// This convention is maintained by the caller.  The order of input cids to the constructor
// must be the same as this canonical order.  It is the caller's responsibility to not
// construct a key with duplicate ids
// TipSetKey is a lightweight value type; passing by pointer is usually unnecessary.

var (
	_ json.Marshaler   = TipSetKey{}
	_ json.Unmarshaler = (*TipSetKey)(nil)
)

var EmptyTSK = TipSetKey{}

// The length of a newBlock header CID in bytes.
var blockHeaderCIDLen int

func init() {
	// hash a large string of zeros so we don't estimate based on inlined CIDs.
	var buf [256]byte
	c, err := abi.CidBuilder.Sum(buf[:])
	if err != nil {
		panic(err)
	}
	blockHeaderCIDLen = len(c.Bytes())
}

// NewTipSetKey builds a new key from a slice of CIDs.
// The CIDs are assumed to be ordered correctly.
func NewTipSetKey(cids ...cid.Cid) TipSetKey {
	encoded := encodeKey(cids)
	return TipSetKey{string(encoded)}
}

// A TipSetKey is an immutable collection of CIDs forming a unique key for a tipset.
// The CIDs are assumed to be distinct and in canonical order. Two keys with the same
// CIDs in a different order are not considered equal.
// TipSetKey is a lightweight value type, and may be compared for equality with ==.
type TipSetKey struct {
	// The internal representation is a concatenation of the bytes of the CIDs, which are
	// self-describing, wrapped as a string.
	// These gymnastics make a TipSetKey usable as a map key.
	// The empty key has value "".
	value string
}

// Cids returns a slice of the CIDs comprising this key.
func (tsk TipSetKey) Cids() []cid.Cid {
	cids, err := decodeKey([]byte(tsk.value))
	if err != nil {
		panic("invalid tipset key: " + err.Error())
	}
	return cids
}

// String returns a human-readable representation of the key.
func (tsk TipSetKey) String() string {
	b := strings.Builder{}
	b.WriteString("{")
	for _, c := range tsk.Cids() {
		b.Write([]byte(fmt.Sprintf(" %s", c.String())))
	}
	b.WriteString(" }")
	return b.String()
}

// Bytes returns a binary representation of the key.
func (tsk TipSetKey) Bytes() []byte {
	return []byte(tsk.value)
}

func (tsk TipSetKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(tsk.Cids())
}

func (tsk *TipSetKey) UnmarshalJSON(b []byte) error {
	var cids []cid.Cid
	if err := json.Unmarshal(b, &cids); err != nil {
		return err
	}
	tsk.value = string(encodeKey(cids))
	return nil
}

func (tsk TipSetKey) IsEmpty() bool {
	return len(tsk.value) == 0
}

// ContainsAll checks if another set is a subset of this one.
// We can assume that the relative order of members of one key is
// maintained in the other since we assume that all ids are sorted
// by corresponding newBlock ticket value.
func (tsk TipSetKey) ContainsAll(other TipSetKey) bool {
	// Since we assume the ids must have the same relative sorting we can
	// perform one pass over this set, advancing the other index whenever the
	// values match.
	cids := tsk.Cids()
	otherCids := other.Cids()
	otherIdx := 0
	for i := 0; i < len(cids) && otherIdx < len(otherCids); i++ {
		if cids[i].Equals(otherCids[otherIdx]) {
			otherIdx++
		}
	}
	// otherIdx is advanced the full length only if every element was found in this set.
	return otherIdx == len(otherCids)
}

// Has checks whether the set contains `id`.
func (tsk TipSetKey) Has(id cid.Cid) bool {
	// Find index of the first CID not less than id.
	for _, cid := range tsk.Cids() {
		if cid == id {
			return true
		}
	}
	return false
}

// Equals checks whether the set contains exactly the same CIDs as another.
func (tsk TipSetKey) Equals(other TipSetKey) bool {
	return tsk.value == other.value
}

// TipSetKeyFromBytes wraps an encoded key, validating correct decoding.
func TipSetKeyFromBytes(encoded []byte) (TipSetKey, error) {
	_, err := decodeKey(encoded)
	if err != nil {
		return TipSetKey{}, err
	}
	return TipSetKey{string(encoded)}, nil
}

func (tsk TipSetKey) Cid() (cid.Cid, error) {
	blk, err := tsk.ToStorageBlock()
	if err != nil {
		return cid.Cid{}, err
	}
	return blk.Cid(), nil
}

func (tsk TipSetKey) ToStorageBlock() (block.Block, error) {
	buf := new(bytes.Buffer)
	if err := tsk.MarshalCBOR(buf); err != nil {
		log.Errorf("failed to marshal ts key as CBOR: %s", tsk)
	}

	cid, err := abi.CidBuilder.Sum(buf.Bytes())
	if err != nil {
		return nil, err
	}

	return block.NewBlockWithCid(buf.Bytes(), cid)
}

// todo: remove after nv18
func (tsk *TipSetKey) V0UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)
	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.Parents: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		cids := make([]cid.Cid, extra)
		for i := 0; i < int(extra); i++ {

			c, err := cbg.ReadCid(br)
			if err != nil {
				return fmt.Errorf("reading cid field t.Parents failed: %v", err)
			}
			cids[i] = c
		}
		tsk.value = string(encodeKey(cids))
	}
	return nil
}

// todo: remove after nv18
func (tsk TipSetKey) V0MarshalCBOR(w io.Writer) error {
	cids := tsk.Cids()
	if len(cids) > cbg.MaxLength {
		return fmt.Errorf("slice value in field t.Parents was too long")
	}
	scratch := make([]byte, 9)

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(cids))); err != nil {
		return err
	}
	for _, v := range cids {
		if err := cbg.WriteCidBuf(scratch, w, v); err != nil {
			return fmt.Errorf("failed writing cid field t.Parents: %v", err)
		}
	}
	return nil
}

func (tsk *TipSetKey) UnmarshalCBOR(reader io.Reader) error {
	cr := cbg.NewCborReader(reader)

	maj, extra, err := cr.ReadHeader()
	if err != nil {
		return err
	}
	defer func() {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
	}()

	if extra > cbg.ByteArrayMaxLen {
		return fmt.Errorf("t.Binary: byte array too large (%d)", extra)
	}
	if maj != cbg.MajByteString {
		return fmt.Errorf("expected byte array")
	}

	b := make([]uint8, extra)

	if _, err := io.ReadFull(cr, b); err != nil {
		return err
	}

	*tsk, err = TipSetKeyFromBytes(b)
	return err
}

func (tsk TipSetKey) MarshalCBOR(writer io.Writer) error {
	if err := cbg.WriteMajorTypeHeader(writer, cbg.MajByteString, uint64(len(tsk.Bytes()))); err != nil {
		return err
	}

	_, err := writer.Write(tsk.Bytes())
	return err
}

func encodeKey(cids []cid.Cid) []byte {
	buffer := new(bytes.Buffer)
	for _, c := range cids {
		// bytes.Buffer.Write() err is documented to be always nil.
		_, _ = buffer.Write(c.Bytes())
	}
	return buffer.Bytes()
}

func decodeKey(encoded []byte) ([]cid.Cid, error) {
	// To avoid reallocation of the underlying array, estimate the number of CIDs to be extracted
	// by dividing the encoded length by the expected CID length.
	estimatedCount := len(encoded) / blockHeaderCIDLen
	cids := make([]cid.Cid, 0, estimatedCount)
	nextIdx := 0
	for nextIdx < len(encoded) {
		nr, c, err := cid.CidFromBytes(encoded[nextIdx:])
		if err != nil {
			return nil, err
		}
		cids = append(cids, c)
		nextIdx += nr
	}
	return cids, nil
}
