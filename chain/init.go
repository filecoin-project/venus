package chain

import (
	"context"
	"encoding/json"

	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
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
	genTipSet, err := consensus.NewTipSet(genesis)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate genesis block")
	}
	chainStore := NewDefaultStore(r.ChainDatastore(), cst, genesis.Cid())

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
