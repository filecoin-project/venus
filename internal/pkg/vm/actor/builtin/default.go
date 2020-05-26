package builtin

import (
	specs "github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/account"
	"github.com/filecoin-project/specs-actors/actors/builtin/cron"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/multisig"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/builtin/reward"
	"github.com/filecoin-project/specs-actors/actors/builtin/system"
	"github.com/filecoin-project/specs-actors/actors/builtin/verifreg"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
)

// DefaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
// Dragons: add the rest of the actors
var DefaultActors = dispatch.NewBuilder().
	Add(specs.InitActorCodeID, &init_.Actor{}).
	Add(specs.AccountActorCodeID, &account.Actor{}).
	Add(specs.MultisigActorCodeID, &multisig.Actor{}).
	Add(specs.PaymentChannelActorCodeID, &paych.Actor{}).
	Add(specs.StoragePowerActorCodeID, &power.Actor{}).
	Add(specs.StorageMarketActorCodeID, &market.Actor{}).
	Add(specs.StorageMinerActorCodeID, &miner.Actor{}).
	Add(specs.SystemActorCodeID, &system.Actor{}).
	Add(specs.RewardActorCodeID, &reward.Actor{}).
	Add(specs.CronActorCodeID, &cron.Actor{}).
	Add(specs.VerifiedRegistryActorCodeID, &verifreg.Actor{}).
	Build()
