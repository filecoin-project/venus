package convert

import (
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"

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
