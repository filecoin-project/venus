package types

import (
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// DefaultHashFunction represents the default hashing function to use
const DefaultHashFunction = mh.BLAKE2B_MIN + 31

// AccountActorCodeCid is the CID of the builtin account actor.
var AccountActorCodeCid *cid.Cid
var StorageMarketActorCodeCid *cid.Cid
var MinerActorCodeCid *cid.Cid

func cidFromString(input string) (*cid.Cid, error) {
	prefix := cid.NewPrefixV1(cid.DagCBOR, DefaultHashFunction)
	return prefix.Sum([]byte(input))
}

func mustCidFromString(s string) *cid.Cid {
	c, err := cidFromString(s)
	if err != nil {
		panic(err)
	}
	return c
}

func init() {
	AccountActorCodeCid = mustCidFromString("accountactor")
	StorageMarketActorCodeCid = mustCidFromString("storagemarket")
	MinerActorCodeCid = mustCidFromString("mineractor")
}
