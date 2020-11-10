package chain

import (
	"context"
	"encoding/json"

	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/genesis"
	"github.com/filecoin-project/venus/internal/pkg/repo"
)

// Init initializes a DefaultSyncer in the given repo.
func Init(ctx context.Context, r repo.Repo, bs bstore.Blockstore, cst cbor.IpldStore, gen genesis.InitFunc) (*Store, error) {
	// TODO the following should be wrapped in the chain.Store or a sub
	// interface.
	// Generate the genesis tipset.
	genesis, err := gen(cst, bs)
	if err != nil {
		return nil, err
	}
	genTipSet, err := block.NewTipSet(genesis)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate genesis block")
	}
	chainStore := NewStore(r.ChainDatastore(), cst, bs, NewStatusReporter(), block.UndefTipSet.Key(), genesis.Cid())

	// Persist the genesis tipset to the repo.
	genTsas := &TipSetMetadata{
		TipSet:          genTipSet,
		TipSetStateRoot: genesis.StateRoot.Cid,
		TipSetReceipts:  genesis.MessageReceipts.Cid,
	}
	if err = chainStore.PutTipSetMetadata(ctx, genTsas); err != nil {
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
