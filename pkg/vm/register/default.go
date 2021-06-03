package register

import (
	actors "github.com/filecoin-project/venus/pkg/specactors"
	"github.com/filecoin-project/venus/pkg/vm/dispatch"

	exported0 "github.com/filecoin-project/specs-actors/actors/builtin/exported"
	exported2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/exported"
	exported3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/exported"
	exported4 "github.com/filecoin-project/specs-actors/v4/actors/builtin/exported"
	exported5 "github.com/filecoin-project/specs-actors/v5/actors/builtin/exported"
)

// DefaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
// Dragons: add the rest of the actors
var DefaultActorBuilder = dispatch.NewBuilder()
var DefaultActors dispatch.CodeLoader

func init() {
	DefaultActorBuilder.AddMany(dispatch.ActorsVersionPredicate(actors.Version0), exported0.BuiltinActors()...)
	DefaultActorBuilder.AddMany(dispatch.ActorsVersionPredicate(actors.Version2), exported2.BuiltinActors()...)
	DefaultActorBuilder.AddMany(dispatch.ActorsVersionPredicate(actors.Version3), exported3.BuiltinActors()...)
	DefaultActorBuilder.AddMany(dispatch.ActorsVersionPredicate(actors.Version4), exported4.BuiltinActors()...)
	DefaultActorBuilder.AddMany(dispatch.ActorsVersionPredicate(actors.Version5), exported5.BuiltinActors()...)
	DefaultActors = DefaultActorBuilder.Build()
}
