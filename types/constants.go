package types

import (
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"
)

// DefaultHashFunction represents the default hashing function to use
const DefaultHashFunction = mh.BLAKE2B_MIN + 31

// AccountActorCodeObj is the code representation of the builtin account actor.
var AccountActorCodeObj ipld.Node

// AccountActorCodeCid is the cid of the above object
var AccountActorCodeCid *cid.Cid

// StorageMarketActorCodeObj is the code representation of the builtin storage market actor.
var StorageMarketActorCodeObj ipld.Node

// StorageMarketActorCodeCid is the cid of the above object
var StorageMarketActorCodeCid *cid.Cid

// PaymentBrokerActorCodeObj is the code representation of the builtin payment broker actor.
var PaymentBrokerActorCodeObj ipld.Node

// PaymentBrokerActorCodeCid is the cid of the above object
var PaymentBrokerActorCodeCid *cid.Cid

// MinerActorCodeObj is the code representation of the builtin miner actor.
var MinerActorCodeObj ipld.Node

// MinerActorCodeCid is the cid of the above object
var MinerActorCodeCid *cid.Cid

// ActorCodeCidTypeNames maps Actor codeCid's to the name of the associated Actor type.
var ActorCodeCidTypeNames = make(map[*cid.Cid]string)

func init() {
	AccountActorCodeObj = dag.NewRawNode([]byte("accountactor"))
	AccountActorCodeCid = AccountActorCodeObj.Cid()
	StorageMarketActorCodeObj = dag.NewRawNode([]byte("storagemarket"))
	StorageMarketActorCodeCid = StorageMarketActorCodeObj.Cid()
	PaymentBrokerActorCodeObj = dag.NewRawNode([]byte("paymentbroker"))
	PaymentBrokerActorCodeCid = PaymentBrokerActorCodeObj.Cid()
	MinerActorCodeObj = dag.NewRawNode([]byte("mineractor"))
	MinerActorCodeCid = MinerActorCodeObj.Cid()

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
