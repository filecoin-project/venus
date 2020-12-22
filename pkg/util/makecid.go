package util

import (
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
)

func MakeCid(i interface{}) (cid.Cid, error) {
	node, err := cbor.WrapObject(i, constants.DefaultHashFunction, -1)
	if err != nil {
		return cid.Undef, err
	}
	return constants.DefaultCidBuilder.Sum(node.RawData())
}
