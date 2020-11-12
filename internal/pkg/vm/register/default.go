package register

import (
	actors "github.com/filecoin-project/venus/internal/pkg/specactors"
	"github.com/filecoin-project/venus/internal/pkg/vm/internal/dispatch"

	exported0 "github.com/filecoin-project/specs-actors/actors/builtin/exported"
	exported2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/exported"
)

// DefaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
// Dragons: add the rest of the actors
var DefaultActorBuilder = dispatch.NewBuilder()
var DefaultActors dispatch.CodeLoader

func init() {
	DefaultActorBuilder.AddMany(dispatch.ActorsVersionPredicate(actors.Version0), exported0.BuiltinActors()...)
	DefaultActorBuilder.AddMany(dispatch.ActorsVersionPredicate(actors.Version2), exported2.BuiltinActors()...)
	DefaultActors = DefaultActorBuilder.Build()
}
