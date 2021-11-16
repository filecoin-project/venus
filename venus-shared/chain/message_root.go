package chain

import (
	"bytes"

	"github.com/filecoin-project/go-state-types/abi"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

type MessageRoot struct {
	BlsRoot   cid.Cid
	SecpkRoot cid.Cid
}

func (mr *MessageRoot) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := mr.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (mr *MessageRoot) SerializeWithCid() (cid.Cid, []byte, error) {
	data, err := mr.Serialize()
	if err != nil {
		return cid.Undef, nil, err
	}

	c, err := abi.CidBuilder.Sum(data)
	if err != nil {
		return cid.Undef, nil, err
	}

	return c, data, nil
}

func (mr *MessageRoot) ToStorageBlock() (blocks.Block, error) {
	c, data, err := mr.SerializeWithCid()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, c)
}

func (mr *MessageRoot) Cid() cid.Cid {
	c, _, err := mr.SerializeWithCid()
	if err != nil {
		panic(err)
	}

	return c
}
