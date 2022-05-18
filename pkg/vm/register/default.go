package register

import (
	exported0 "github.com/filecoin-project/specs-actors/actors/builtin/exported"
	exported2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/exported"
	exported3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/exported"
	exported4 "github.com/filecoin-project/specs-actors/v4/actors/builtin/exported"
	exported5 "github.com/filecoin-project/specs-actors/v5/actors/builtin/exported"
	exported6 "github.com/filecoin-project/specs-actors/v6/actors/builtin/exported"
	exported7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/exported"
	exported8 "github.com/filecoin-project/specs-actors/v8/actors/builtin/exported"

	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	"github.com/filecoin-project/venus/venus-shared/actors"
)

// DefaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
// Dragons: add the rest of the actors
var DefaultActorBuilder = dispatch.NewBuilder()
var DefaultActors dispatch.CodeLoader

func init() {
	DefaultActorBuilder.AddMany(actors.Version0, dispatch.ActorsVersionPredicate(actors.Version0), exported0.BuiltinActors()...)
	DefaultActorBuilder.AddMany(actors.Version2, dispatch.ActorsVersionPredicate(actors.Version2), exported2.BuiltinActors()...)
	DefaultActorBuilder.AddMany(actors.Version3, dispatch.ActorsVersionPredicate(actors.Version3), exported3.BuiltinActors()...)
	DefaultActorBuilder.AddMany(actors.Version4, dispatch.ActorsVersionPredicate(actors.Version4), exported4.BuiltinActors()...)
	DefaultActorBuilder.AddMany(actors.Version5, dispatch.ActorsVersionPredicate(actors.Version5), exported5.BuiltinActors()...)
	DefaultActorBuilder.AddMany(actors.Version6, dispatch.ActorsVersionPredicate(actors.Version6), exported6.BuiltinActors()...)
	DefaultActorBuilder.AddMany(actors.Version7, dispatch.ActorsVersionPredicate(actors.Version7), exported7.BuiltinActors()...)
	DefaultActorBuilder.AddMany(actors.Version8, dispatch.ActorsVersionPredicate(actors.Version8), exported8.BuiltinActors()...)
	DefaultActors = DefaultActorBuilder.Build()
}
