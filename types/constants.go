package types

import (
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmZo5avr9dhVVRzcpKnU9ZGQuPaU62pbufUHXBNB7GwLzQ/go-basex"
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

// PaymentBrokerActorCodeCid is the CID of the builtin payment broker actor.
var PaymentBrokerActorCodeCid *cid.Cid

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

// ActorCodeCidTypeNames maps Actor codeCid's to the name of the associated Actor type.
var ActorCodeCidTypeNames = make(map[*cid.Cid]string)

func init() {
	AccountActorCodeCid = mustCidFromString("accountactor")
	StorageMarketActorCodeCid = mustCidFromString("storagemarket")
	PaymentBrokerActorCodeCid = mustCidFromString("paymentbroker")
	MinerActorCodeCid = mustCidFromString("mineractor")

	// New Actors need to be added here.
	// TODO: Make this work with reflection -- but note that nasty import cycles lie on that path.
	// This is good enough for now.
	ActorCodeCidTypeNames[AccountActorCodeCid] = "AccountActor"
	ActorCodeCidTypeNames[StorageMarketActorCodeCid] = "StorageMarketActor"
	ActorCodeCidTypeNames[PaymentBrokerActorCodeCid] = "PaymentBrokerActor"
	ActorCodeCidTypeNames[MinerActorCodeCid] = "MinerActor"
}

// ActorCodeTypeName returns the (string) name of the Go type of the actor with cid, code.
func ActorCodeTypeName(code *cid.Cid) string {
	if code == nil {
		return "EmptyActor"
	}

	name, ok := ActorCodeCidTypeNames[code]
	if ok {
		return name
	}
	return "UnknownActor"
}
