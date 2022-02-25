package node

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// readGenesisCid is a helper function that queries the provided datastore for
// an entry with the genesisKey cid, returning if found.
func readGenesisCid(ctx context.Context, chainDs datastore.Datastore, bs blockstoreutil.Blockstore) (types.BlockHeader, error) {
	bb, err := chainDs.Get(ctx, chain.GenesisKey)
	if err != nil {
		return types.BlockHeader{}, errors.Wrap(err, "failed to read genesisKey")
	}

	var c cid.Cid
	err = json.Unmarshal(bb, &c)
	if err != nil {
		return types.BlockHeader{}, errors.Wrap(err, "failed to cast genesisCid")
	}

	blkRawData, err := bs.Get(ctx, c)
	if err != nil {
		return types.BlockHeader{}, errors.Wrap(err, "failed to read genesis block")
	}

	var blk types.BlockHeader
	err = blk.UnmarshalCBOR(bytes.NewReader(blkRawData.RawData()))
	if err != nil {
		return types.BlockHeader{}, errors.Wrap(err, "failed to unmarshal genesis block")
	}
	return blk, nil
}
