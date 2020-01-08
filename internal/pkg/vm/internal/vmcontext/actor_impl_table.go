package vmcontext

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/ipfs/go-cid"
)

type prodActorImplTable struct {
	actors builtin.Actors
}

// NewProdActorImplTable creates a lookup table with the production implementations of actors.
func NewProdActorImplTable() ActorImplLookup {
	return &prodActorImplTable{
		actors: builtin.DefaultActors,
	}
}

var _ ActorImplLookup = (*prodActorImplTable)(nil)

func (t *prodActorImplTable) GetActorImpl(code cid.Cid, epoch types.BlockHeight) (dispatch.ExecutableActor, error) {
	// TODO: move the table over here, and have it support height lookup (#3360)
	return t.actors.GetActorCode(code, 0)
}
