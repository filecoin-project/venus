package builtin

import (
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	account0 "github.com/filecoin-project/specs-actors/actors/builtin/account"
	cron0 "github.com/filecoin-project/specs-actors/actors/builtin/cron"
	init0 "github.com/filecoin-project/specs-actors/actors/builtin/init"
	market0 "github.com/filecoin-project/specs-actors/actors/builtin/market"
	miner0 "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	multisig0 "github.com/filecoin-project/specs-actors/actors/builtin/multisig"
	paych0 "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	power0 "github.com/filecoin-project/specs-actors/actors/builtin/power"
	reward0 "github.com/filecoin-project/specs-actors/actors/builtin/reward"
	system0 "github.com/filecoin-project/specs-actors/actors/builtin/system"
	verifreg0 "github.com/filecoin-project/specs-actors/actors/builtin/verifreg"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
)

// DefaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
// Dragons: add the rest of the actors
var DefaultActors = dispatch.NewBuilder().
	Add(builtin0.SystemActorCodeID, &system0.Actor{}).
	Add(builtin0.InitActorCodeID, &init0.Actor{}).
	Add(builtin0.RewardActorCodeID, &reward0.Actor{}).
	Add(builtin0.CronActorCodeID, &cron0.Actor{}).
	Add(builtin0.AccountActorCodeID, &account0.Actor{}).
	Add(builtin0.StoragePowerActorCodeID, &power0.Actor{}).
	Add(builtin0.StorageMarketActorCodeID, &market0.Actor{}).
	Add(builtin0.StorageMinerActorCodeID, &miner0.Actor{}).
	Add(builtin0.MultisigActorCodeID, &multisig0.Actor{}).
	Add(builtin0.PaymentChannelActorCodeID, &paych0.Actor{}).
	Add(builtin0.VerifiedRegistryActorCodeID, &verifreg0.Actor{}).
	Build()
