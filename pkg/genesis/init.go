package genesis

import (
	"context"
	"encoding/json"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/types"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/repo"
)

// Init initializes a DefaultSyncer in the given repo.
func Init(ctx context.Context, r repo.Repo, bs blockstoreutil.Blockstore, cst cbor.IpldStore, gen InitFunc) (*chain.Store, error) {
	// TODO the following should be wrapped in the chain.Store or a sub
	// interface.
	// Generate the genesis tipset.
	genesis, err := gen(cst, bs)
	if err != nil {
		return nil, err
	}
	genTipSet, err := types.NewTipSet([]*types.BlockHeader{genesis})
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate genesis block")
	}
	//todo give fork params

	chainStore := chain.NewStore(r.ChainDatastore(), bs, genesis.Cid(), chain.NewCirculatingSupplyCalculator(bs, genesis.Cid(), config.DefaultForkUpgradeParam))

	// Persist the genesis tipset to the repo.
	genTsas := &chain.TipSetMetadata{
		TipSet:          genTipSet,
		TipSetStateRoot: genesis.ParentStateRoot,
		TipSetReceipts:  genesis.ParentMessageReceipts,
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

	if err = r.ChainDatastore().Put(ctx, chain.GenesisKey, val); err != nil {
		return nil, errors.Wrap(err, "failed to persist genesis cid")
	}

	return chainStore, nil
}
