package builtin

import (
	"fmt"
	actors "github.com/filecoin-project/go-filecoin/internal/pkg/specactors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"

	exported0 "github.com/filecoin-project/specs-actors/actors/builtin/exported"
	exported2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/exported"
)

// DefaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
// Dragons: add the rest of the actors
var DefaultActorBuilder = dispatch.NewBuilder()
var DefaultActors dispatch.CodeLoader

func init() {
	xxx := exported0.BuiltinActors()
	yyy := exported2.BuiltinActors()
	for _, xx := range xxx {
		fmt.Println(xx.Code())
	}
	for _, xx := range yyy {
		fmt.Println(xx.Code())
	}
	DefaultActorBuilder.AddMany(dispatch.ActorsVersionPredicate(actors.Version0), exported0.BuiltinActors()...)
	DefaultActorBuilder.AddMany(dispatch.ActorsVersionPredicate(actors.Version2), exported2.BuiltinActors()...)
	DefaultActors = DefaultActorBuilder.Build()
}
