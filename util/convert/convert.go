package convert

import (
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

// ToCid gets the Cid for the argument passed in
func ToCid(object interface{}) (cid.Cid, error) {
	cborNode, err := cbor.WrapObject(object, types.DefaultHashFunction, -1)
	if err != nil {
		return cid.Cid{}, errors.Wrap(err, "failed to get cid of proposal")
	}
	return cborNode.Cid(), nil
}
