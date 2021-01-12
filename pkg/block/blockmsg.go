package block

import (
	"bytes"

	"github.com/ipfs/go-cid"
)

type BlockMsg struct { // nolint: golint
	Header        *Block
	BlsMessages   []cid.Cid
	SecpkMessages []cid.Cid
}

func (bm *BlockMsg) Cid() cid.Cid {
	return bm.Header.Cid()
}

func (bm *BlockMsg) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bm.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
