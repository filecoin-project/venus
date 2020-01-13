package convert

import (
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// ToCid gets the Cid for the argument passed in
func ToCid(object interface{}) (cid.Cid, error) {
	cborNode, err := cbor.WrapObject(object, types.DefaultHashFunction, -1)
	if err != nil {
		return cid.Cid{}, errors.Wrap(err, "failed to get cid of proposal")
	}
	return cborNode.Cid(), nil
}

// To32ByteArray creates a 32-byte array of NUL bytes, and then copies the first
// 32 bytes from input into that array and returns the array.
func To32ByteArray(input []byte) [32]byte {
	var output [32]byte
	copy(output[:], input)

	return output
}
