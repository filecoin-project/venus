package types

import (
	"gx/ipfs/QmZo5avr9dhVVRzcpKnU9ZGQuPaU62pbufUHXBNB7GwLzQ/go-basex"
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// DefaultHashFunction represents the default hashing function to use
const DefaultHashFunction = mh.BLAKE2B_MIN + 31

// AddressHashLength is the length of an the hash part of the address in bytes.
const AddressHashLength = 20

// AddressLength is the lengh of a full address in bytes.
const AddressLength = 1 + 1 + AddressHashLength

// AddressVersion is the current version of the address format.
const AddressVersion byte = 0

// Base32Charset is the character set used for base32 encoding in addresses.
const Base32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

// Base32CharsetReverse is the reverse character set. It maps ASCII byte -> Base32Charset index on [0,31].
var Base32CharsetReverse = [128]int8{
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	15, -1, 10, 17, 21, 20, 26, 30, 7, 5, -1, -1, -1, -1, -1, -1,
	-1, 29, -1, 24, 13, 25, 9, 8, 23, -1, 18, 22, 31, 27, 19, -1,
	1, 0, 3, 16, 11, 28, 12, 14, 6, 4, 2, -1, -1, -1, -1, -1,
	-1, 29, -1, 24, 13, 25, 9, 8, 23, -1, 18, 22, 31, 27, 19, -1,
	1, 0, 3, 16, 11, 28, 12, 14, 6, 4, 2, -1, -1, -1, -1, -1,
}

// Base32 is a basex instance using the Base32Charset.
var Base32 = basex.NewAlphabet(Base32Charset)

// AccountActorCodeCid is the CID of the builtin account actor.
var AccountActorCodeCid *cid.Cid

// StorageMarketActorCodeCid is the CID of the builtin storage market actor.
var StorageMarketActorCodeCid *cid.Cid

// MinerActorCodeCid is the CID of the builtin miner actor.
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
