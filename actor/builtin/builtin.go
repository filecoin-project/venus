// Package builtin implements the predefined actors in Filecoin.
package builtin

import (
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
)

// Actors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
var Actors = map[cid.Cid]exec.ExecutableActor{}

func init() {
	// Instance Actors
	Actors[types.AccountActorCodeCid] = &account.Actor{}
	Actors[types.StorageMarketActorCodeCid] = &storagemarket.Actor{}
	Actors[types.PaymentBrokerActorCodeCid] = &paymentbroker.Actor{}
	Actors[types.MinerActorCodeCid] = &miner.Actor{}
	Actors[types.BootstrapMinerActorCodeCid] = &miner.Actor{Bootstrap: true}
}
