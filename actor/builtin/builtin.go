// Package builtin implements the predefined actors in Filecoin.
package builtin

import (
	"fmt"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	cid "github.com/ipfs/go-cid"
)

type codeVersion struct {
	code    cid.Cid
	version uint64
}

type Actors struct {
	actors map[codeVersion]exec.ExecutableActor
}

func (ba Actors) GetBuiltinActorCode(code cid.Cid, version uint64) (exec.ExecutableActor, error) {
	if !code.Defined() {
		return nil, fmt.Errorf("missing code")
	}
	actor, ok := ba.actors[codeVersion{code: code, version: version}]
	if !ok {
		return nil, fmt.Errorf("unknown code: %s", code.String())
	}
	return actor, nil
}

type BuiltinActorsBuilder struct {
	actors map[codeVersion]exec.ExecutableActor
}

func NewBuiltinActorsBuilder() *BuiltinActorsBuilder {
	return &BuiltinActorsBuilder{actors: map[codeVersion]exec.ExecutableActor{}}
}

func BuiltinActorsExtender(actors Actors) *BuiltinActorsBuilder {
	newActors := make(map[codeVersion]exec.ExecutableActor, len(actors.actors))
	for k, v := range actors.actors {
		newActors[k] = v
	}
	return &BuiltinActorsBuilder{actors: newActors}
}

func (bab *BuiltinActorsBuilder) Add(c cid.Cid, version uint64, actor exec.ExecutableActor) *BuiltinActorsBuilder {
	bab.actors[codeVersion{code: c, version: version}] = actor
	return bab
}

func (bab *BuiltinActorsBuilder) Build() Actors {
	return Actors{actors: bab.actors}
}

// DefaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
var DefaultActors = NewBuiltinActorsBuilder().
	Add(types.AccountActorCodeCid, 0, &account.Actor{}).
	Add(types.StorageMarketActorCodeCid, 0, &storagemarket.Actor{}).
	Add(types.PaymentBrokerActorCodeCid, 0, &paymentbroker.Actor{}).
	Add(types.MinerActorCodeCid, 0, &miner.Actor{}).
	Add(types.BootstrapMinerActorCodeCid, 0, &miner.Actor{Bootstrap: true}).
	Add(types.InitActorCodeCid, 0, &initactor.Actor{}).
	Build()
