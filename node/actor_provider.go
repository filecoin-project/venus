package node

import (
	"context"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/pkg/errors"
)

// defaultActorProvider reads actor state out of the IPLD HAMT store.
// This is wiring to provide actor state to the outbox.
type defaultActorProvider struct {
	chain chainStore
	state *hamt.CborIpldStore
}

type chainStore interface {
	GetTipSetStateRoot(tsKey types.SortedCidSet) (cid.Cid, error)
}

func newDefaultActorProvider(chain chainStore, state *hamt.CborIpldStore) *defaultActorProvider {
	return &defaultActorProvider{chain, state}
}

func (ap defaultActorProvider) GetActor(ctx context.Context, tipset types.SortedCidSet, addr address.Address) (*actor.Actor, error) {
	stateCid, err := ap.chain.GetTipSetStateRoot(tipset)
	if err != nil {
		return nil, err
	}
	tree, err := state.LoadStateTree(ctx, ap.state, stateCid, builtin.Actors)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load latest state")
	}

	actr, err := tree.GetActor(ctx, addr)
	if err != nil {
		return nil, errors.Wrapf(err, "no actor at address %s", addr)
	}
	return actr, nil
}
