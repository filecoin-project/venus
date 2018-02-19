package types

import (
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// DefaultHashFunction represents the default hashing function to use
const DefaultHashFunction = mh.BLAKE2B_MIN + 31

// TokenActorCid is the CID of the builtin token actor.
var TokenActorCid *cid.Cid

// AccountActorCid is the CID of the builtin account actor.
var AccountActorCid *cid.Cid

func cidFromString(input string) (*cid.Cid, error) {
	prefix := cid.NewPrefixV1(cid.DagCBOR, DefaultHashFunction)
	return prefix.Sum([]byte(input))
}

func init() {
	token, err := cidFromString("tokenactor")
	if err != nil {
		panic(err)
	}
	TokenActorCid = token

	acc, err := cidFromString("accountactor")
	if err != nil {
		panic(err)
	}
	AccountActorCid = acc
}
