package builtin

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/power"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	specs "github.com/filecoin-project/specs-actors/actors/builtin"
)

// DefaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
var DefaultActors = dispatch.NewBuilder().
	Add(specs.InitActorCodeID, &initactor.Actor{}).
	Add(specs.AccountActorCodeID, &account.Actor{}).
	Add(specs.StoragePowerActorCodeID, &power.Actor{}).
	Add(specs.StorageMinerActorCodeID, &miner.Actor{}).
	Add(types.BootstrapMinerActorCodeCid, &miner.Actor{Bootstrap: true}).
	Build()
