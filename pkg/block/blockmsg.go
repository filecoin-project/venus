package block

import (
	"github.com/filecoin-project/venus/pkg/encoding"

	"github.com/ipfs/go-cid"
)

type BlockMsg struct { // nolint: golint
	Header        *Block
	BlsMessages   []cid.Cid
	SecpkMessages []cid.Cid
}

func DecodeBlockMsg(b []byte) (*BlockMsg, error) {
	var bm BlockMsg
	if err := encoding.Decode(b, &bm); err != nil {
		return nil, err
	}

	return &bm, nil
}

func (bm *BlockMsg) Cid() cid.Cid {
	return bm.Header.Cid()
}

func (bm *BlockMsg) Serialize() ([]byte, error) {
	bytes, err := encoding.Encode(bm)
	if err != nil {
		return nil, err
	}
	return bytes, nil

}
