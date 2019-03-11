package actr

import (
	"context"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
)

// ChainReadStore is the subset of chain.ReadStore that Actor needs.
type ChainReadStore interface {
	LatestState(ctx context.Context) (state.Tree, error)
}

// Actor provides a uniform interface to actors from the chain state
type Actor struct {
	chainReader ChainReadStore
}

// NewActor creates a new Actor struct
func NewActor(cr ChainReadStore) *Actor {
	return &Actor{
		chainReader: cr,
	}
}

// Get returns an actor from the latest state on the chain
func (a *Actor) Get(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	state, err := a.chainReader.LatestState(ctx)
	if err != nil {
		return nil, err
	}
	return state.GetActor(ctx, addr)
}

// Ls returns a slice of actors from the latest state on the chain
func (a *Actor) Ls(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
	st, err := a.chainReader.LatestState(ctx)
	if err != nil {
		return nil, err
	}
	return state.GetAllActors(st), nil
}
