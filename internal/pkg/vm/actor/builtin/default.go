package builtin

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/power"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
)

// DefaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
var DefaultActors = dispatch.NewBuilder().
	Add(types.InitActorCodeCid, &initactor.Actor{}).
	Add(types.AccountActorCodeCid, &account.Actor{}).
	Add(types.PowerActorCodeCid, &power.Actor{}).
	Add(types.MinerActorCodeCid, &miner.Actor{}).
	Add(types.BootstrapMinerActorCodeCid, &miner.Actor{Bootstrap: true}).
	Build()
