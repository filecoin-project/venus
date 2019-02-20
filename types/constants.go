package types

import (
	dag "gx/ipfs/QmNRAuGmvnVw8urHkUZQirhu42VTiZjVWASa2aTznEMmpP/go-merkledag"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"
)

// DefaultHashFunction represents the default hashing function to use
const DefaultHashFunction = mh.BLAKE2B_MIN + 31

// AccountActorCodeObj is the code representation of the builtin account actor.
var AccountActorCodeObj ipld.Node

// AccountActorCodeCid is the cid of the above object
var AccountActorCodeCid cid.Cid

// StorageMarketActorCodeObj is the code representation of the builtin storage market actor.
var StorageMarketActorCodeObj ipld.Node

// StorageMarketActorCodeCid is the cid of the above object
var StorageMarketActorCodeCid cid.Cid

// PaymentBrokerActorCodeObj is the code representation of the builtin payment broker actor.
var PaymentBrokerActorCodeObj ipld.Node

// PaymentBrokerActorCodeCid is the cid of the above object
var PaymentBrokerActorCodeCid cid.Cid

// MinerActorCodeObj is the code representation of the builtin miner actor.
var MinerActorCodeObj ipld.Node

// MinerActorCodeCid is the cid of the above object
var MinerActorCodeCid cid.Cid

// BootstrapMinerActorCodeObj is the code representation of the bootstrap miner actor.
var BootstrapMinerActorCodeObj ipld.Node

// BootstrapMinerActorCodeCid is the cid of the above object
var BootstrapMinerActorCodeCid cid.Cid

// ActorCodeCidTypeNames maps Actor codeCid's to the name of the associated Actor type.
var ActorCodeCidTypeNames = make(map[cid.Cid]string)

func init() {
	AccountActorCodeObj = dag.NewRawNode([]byte("accountactor"))
	AccountActorCodeCid = AccountActorCodeObj.Cid()
	StorageMarketActorCodeObj = dag.NewRawNode([]byte("storagemarket"))
	StorageMarketActorCodeCid = StorageMarketActorCodeObj.Cid()
	PaymentBrokerActorCodeObj = dag.NewRawNode([]byte("paymentbroker"))
	PaymentBrokerActorCodeCid = PaymentBrokerActorCodeObj.Cid()
	MinerActorCodeObj = dag.NewRawNode([]byte("mineractor"))
	MinerActorCodeCid = MinerActorCodeObj.Cid()
	BootstrapMinerActorCodeObj = dag.NewRawNode([]byte("bootstrapmineractor"))
	BootstrapMinerActorCodeCid = BootstrapMinerActorCodeObj.Cid()

	// New Actors need to be added here.
	// TODO: Make this work with reflection -- but note that nasty import cycles lie on that path.
	// This is good enough for now.
	ActorCodeCidTypeNames[AccountActorCodeCid] = "AccountActor"
	ActorCodeCidTypeNames[StorageMarketActorCodeCid] = "StorageMarketActor"
	ActorCodeCidTypeNames[PaymentBrokerActorCodeCid] = "PaymentBrokerActor"
	ActorCodeCidTypeNames[MinerActorCodeCid] = "MinerActor"
	ActorCodeCidTypeNames[BootstrapMinerActorCodeCid] = "MinerActor"
}

// ActorCodeTypeName returns the (string) name of the Go type of the actor with cid, code.
func ActorCodeTypeName(code cid.Cid) string {
	if !code.Defined() {
		return "EmptyActor"
	}

	name, ok := ActorCodeCidTypeNames[code]
	if ok {
		return name
	}
	return "UnknownActor"
}
