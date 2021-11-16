package chain

import (
	"bytes"

	"github.com/ipfs/go-cid"
)

type BlockMsg struct { // nolint: golint
	Header        *BlockHeader
	BlsMessages   []cid.Cid
	SecpkMessages []cid.Cid
}

// Cid return block cid
func (bm *BlockMsg) Cid() cid.Cid {
	return bm.Header.Cid()
}

// Serialize return blockmsg binary
func (bm *BlockMsg) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bm.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
