package types

import (
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// DefaultHashFunction represents the default hashing function to use
const DefaultHashFunction = mh.BLAKE2B_MIN + 31

// BootstrapMinerActorCodeCid is the cid of the above object
// Dragons: this needs to be deleted once we bring the new actors in
var BootstrapMinerActorCodeCid cid.Cid

func init() {
	builder := cid.V1Builder{Codec: cid.Raw, MhType: mh.IDENTITY}
	makeBuiltin := func(s string) cid.Cid {
		c, err := builder.Sum([]byte(s))
		if err != nil {
			panic(err)
		}
		return c
	}

	BootstrapMinerActorCodeCid = makeBuiltin("fil/1/bootstrap")
}

// ActorCodeTypeName returns the (string) name of the Go type of the actor with cid, code.
// Dragons: remove or we migh want to add support for it from the actors repo
func ActorCodeTypeName(code cid.Cid) string {
	if !code.Defined() {
		return "EmptyActor"
	}

	names := map[cid.Cid]string{
		builtin.InitActorCodeID:           "InitActor",
		builtin.StoragePowerActorCodeID:   "PowerActor",
		builtin.StorageMarketActorCodeID:  "StorageMarketActor",
		builtin.AccountActorCodeID:        "AccountActor",
		builtin.StorageMinerActorCodeID:   "MinerActor",
		BootstrapMinerActorCodeCid:        "MinerActor",
		builtin.CronActorCodeID:           "CronActor",
		builtin.MultisigActorCodeID:       "MultisigActor",
		builtin.PaymentChannelActorCodeID: "PaymentChannelActor",
		builtin.RewardActorCodeID:         "RewardActor",
	}
	name, ok := names[code]
	if !ok {
		return "UnknownActor"
	}
	return name
}
