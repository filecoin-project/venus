package types

import (
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
	mh "github.com/multiformats/go-multihash"
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

// PowerActorCodeObj is the code representation of the builtin power actor
var PowerActorCodeObj ipld.Node

// PowerActorCodeCid is the cid of the above object
var PowerActorCodeCid cid.Cid

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

// InitActorCodeObj is the code representation of the builtin init actor.
var InitActorCodeObj ipld.Node

// InitActorCodeCid is the cid of the above object
var InitActorCodeCid cid.Cid

// ActorCodeCidTypeNames maps Actor codeCid's to the name of the associated Actor type.
var ActorCodeCidTypeNames = make(map[cid.Cid]string)

func init() {
	AccountActorCodeObj = dag.NewRawNode([]byte("accountactor"))
	AccountActorCodeCid = AccountActorCodeObj.Cid()
	StorageMarketActorCodeObj = dag.NewRawNode([]byte("storagemarket"))
	StorageMarketActorCodeCid = StorageMarketActorCodeObj.Cid()
	PowerActorCodeObj = dag.NewRawNode([]byte("power"))
	PowerActorCodeCid = PowerActorCodeObj.Cid()
	PaymentBrokerActorCodeObj = dag.NewRawNode([]byte("paymentbroker"))
	PaymentBrokerActorCodeCid = PaymentBrokerActorCodeObj.Cid()
	MinerActorCodeObj = dag.NewRawNode([]byte("mineractor"))
	MinerActorCodeCid = MinerActorCodeObj.Cid()
	BootstrapMinerActorCodeObj = dag.NewRawNode([]byte("bootstrapmineractor"))
	BootstrapMinerActorCodeCid = BootstrapMinerActorCodeObj.Cid()
	InitActorCodeObj = dag.NewRawNode([]byte("initactor"))
	InitActorCodeCid = InitActorCodeObj.Cid()

	// New Actors need to be added here.
	// TODO: Make this work with reflection -- but note that nasty import cycles lie on that path.
	// This is good enough for now.
	ActorCodeCidTypeNames[AccountActorCodeCid] = "AccountActor"
	ActorCodeCidTypeNames[StorageMarketActorCodeCid] = "StorageMarketActor"
	ActorCodeCidTypeNames[PowerActorCodeCid] = "PowerActor"
	ActorCodeCidTypeNames[PaymentBrokerActorCodeCid] = "PaymentBrokerActor"
	ActorCodeCidTypeNames[MinerActorCodeCid] = "MinerActor"
	ActorCodeCidTypeNames[BootstrapMinerActorCodeCid] = "MinerActor"
	ActorCodeCidTypeNames[InitActorCodeCid] = "InitActor"
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
