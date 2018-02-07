package types

import (
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// DefaultHashFunction represents the default hashing function to use
const DefaultHashFunction = mh.BLAKE2B_MIN + 31

var AccountActorCid *cid.Cid

func init() {
	prefix := cid.NewPrefixV1(cid.DagCBOR, DefaultHashFunction)
	c, err := prefix.Sum([]byte("accountactor"))
	if err != nil {
		panic(err)
	}

	AccountActorCid = c
}
