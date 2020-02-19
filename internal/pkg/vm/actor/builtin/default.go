package builtin

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	specs "github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/account"
	notinit "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
)

// DefaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
// Dragons: add the rest of the actors
var DefaultActors = dispatch.NewBuilder().
	Add(specs.InitActorCodeID, &notinit.Actor{}).
	Add(specs.AccountActorCodeID, &account.Actor{}).
	Add(specs.StoragePowerActorCodeID, &power.Actor{}).
	Add(specs.StorageMinerActorCodeID, &miner.Actor{}).
	Build()
