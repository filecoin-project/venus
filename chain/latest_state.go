package chain

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

type latestStateChainReader interface {
	GetHead() types.SortedCidSet
	GetTipSetStateRoot(tsKey types.SortedCidSet) (cid.Cid, error)
}

// LatestState gets the latest state from the state Store.
func LatestState(ctx context.Context, store latestStateChainReader, stateStore *hamt.CborIpldStore) (state.Tree, error) {
	stateCid, err := store.GetTipSetStateRoot(store.GetHead())
	if err != nil {
		return nil, err
	}
	return state.LoadStateTree(ctx, stateStore, stateCid, builtin.Actors)
}
