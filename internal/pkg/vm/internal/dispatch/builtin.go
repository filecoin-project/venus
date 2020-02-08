package dispatch

import (
	"github.com/filecoin-project/specs-actors/actors/builtin"
	notinit "github.com/filecoin-project/specs-actors/actors/builtin/init"
)

// DefaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
var DefaultActors = NewBuilder().
	Add(builtin.InitActorCodeID, &notinit.Actor{}).
	Build()
