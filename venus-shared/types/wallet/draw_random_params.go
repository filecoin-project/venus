package wallet

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/crypto"
	fcbor "github.com/fxamacker/cbor/v2"
	"github.com/minio/blake2b-simd"
)

type DrawRandomParams struct {
	Rbase   []byte
	Pers    crypto.DomainSeparationTag
	Round   abi.ChainEpoch
	Entropy []byte
}

// return store.DrawRandomness(dr.Rbase, dr.Pers, dr.Round, dr.Entropy)
func (dr *DrawRandomParams) SignBytes() ([]byte, error) {
	h := blake2b.New256()
	if err := binary.Write(h, binary.BigEndian, int64(dr.Pers)); err != nil {
		return nil, fmt.Errorf("deriving randomness: %w", err)
	}
	VRFDigest := blake2b.Sum256(dr.Rbase)
	_, err := h.Write(VRFDigest[:])
	if err != nil {
		return nil, fmt.Errorf("hashing VRFDigest: %w", err)
	}
	if err := binary.Write(h, binary.BigEndian, dr.Round); err != nil {
		return nil, fmt.Errorf("deriving randomness: %w", err)
	}
	_, err = h.Write(dr.Entropy)
	if err != nil {
		return nil, fmt.Errorf("hashing entropy: %w", err)
	}

	return h.Sum(nil), nil
}

func (dr *DrawRandomParams) MarshalCBOR(w io.Writer) error {
	data, err := fcbor.Marshal(dr)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (dr *DrawRandomParams) UnmarshalCBOR(r io.Reader) error {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return fcbor.Unmarshal(data, dr)
}

var _ = cbor.Unmarshaler((*DrawRandomParams)(nil))
var _ = cbor.Marshaler((*DrawRandomParams)(nil))
