// Package builtin implements the predefined actors in Filecoin.
package builtin

import (
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
)

// Actors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
var Actors = map[string]exec.ExecutableActor{}

func init() {
	// Instance Actors
	Actors[types.AccountActorCodeCid.KeyString()] = &account.Actor{}
	Actors[types.StorageMarketActorCodeCid.KeyString()] = &storagemarket.Actor{}
	Actors[types.PaymentBrokerActorCodeCid.KeyString()] = &paymentbroker.Actor{}
	Actors[types.MinerActorCodeCid.KeyString()] = &miner.Actor{}
}
