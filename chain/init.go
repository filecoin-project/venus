package chain

import (
	"context"
	"encoding/json"

	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
)

// Init initializes a DefaultSycner in the given repo.
func Init(ctx context.Context, r repo.Repo, bs bstore.Blockstore, cst *hamt.CborIpldStore, gen consensus.GenesisInitFunc) (*DefaultStore, error) {
	// TODO the following should be wrapped in the chain.Store or a sub
	// interface.
	// Generate the genesis tipset.
	genesis, err := gen(cst, bs)
	if err != nil {
		return nil, err
	}
	genTipSet, err := types.NewTipSet(genesis)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate genesis block")
	}
	chainStore := NewDefaultStore(r.ChainDatastore(), genesis.Cid())

	// Persist the genesis tipset to the repo.
	genTsas := &TipSetAndState{
		TipSet:          genTipSet,
		TipSetStateRoot: genesis.StateRoot,
	}
	if err = chainStore.PutTipSetAndState(ctx, genTsas); err != nil {
		return nil, errors.Wrap(err, "failed to put genesis block in chain store")
	}
	if err = chainStore.SetHead(ctx, genTipSet); err != nil {
		return nil, errors.Wrap(err, "failed to persist genesis block in chain store")
	}
	// Persist the genesis cid to the repo.
	val, err := json.Marshal(genesis.Cid())
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal genesis cid")
	}
	if err = r.Datastore().Put(GenesisKey, val); err != nil {
		return nil, errors.Wrap(err, "failed to persist genesis cid")
	}

	return chainStore, nil
}
