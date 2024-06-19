package chain

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/venus-shared/types"
	cid "github.com/ipfs/go-cid"
)

type ChainReader interface {
	GetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error)
	GetHead() *types.TipSet
	StateView(ctx context.Context, ts *types.TipSet) (*appstate.View, error)
	GetTipSetStateRoot(context.Context, *types.TipSet) (cid.Cid, error)
	GetTipSetReceiptsRoot(context.Context, *types.TipSet) (cid.Cid, error)
	GetGenesisBlock(context.Context) (*types.BlockHeader, error)
	GetLatestBeaconEntry(context.Context, *types.TipSet) (*types.BeaconEntry, error)
	GetTipSetByHeight(context.Context, *types.TipSet, abi.ChainEpoch, bool) (*types.TipSet, error)
	GetLookbackTipSetForRound(ctx context.Context, ts *types.TipSet, round abi.ChainEpoch, version network.Version) (*types.TipSet, cid.Cid, error)
	GetTipsetMetadata(context.Context, *types.TipSet) (*TipSetMetadata, error)
	PutTipSetMetadata(context.Context, *TipSetMetadata) error
	Weight(context.Context, *types.TipSet) (big.Int, error)

	ICirculatingSupplyCalcualtor
}

type chainReader struct {
	*Store
	circulatingSupplyCalculator ICirculatingSupplyCalcualtor
}

func ChainReaderWrapper(store *Store, circulatingSupplyCalculator ICirculatingSupplyCalcualtor) ChainReader {
	return &chainReader{
		Store:                       store,
		circulatingSupplyCalculator: circulatingSupplyCalculator,
	}
}

// GetCirculatingSupply implements ChainReader.
func (c *chainReader) GetCirculatingSupply(ctx context.Context, height abi.ChainEpoch, st tree.Tree) (big.Int, error) {
	return c.circulatingSupplyCalculator.GetCirculatingSupply(ctx, height, st)
}

// GetCirculatingSupplyDetailed implements ChainReader.
func (c *chainReader) GetCirculatingSupplyDetailed(ctx context.Context, height abi.ChainEpoch, st tree.Tree) (types.CirculatingSupply, error) {
	return c.circulatingSupplyCalculator.GetCirculatingSupplyDetailed(ctx, height, st)
}

// GetFilVested implements ChainReader.
func (c *chainReader) GetFilVested(ctx context.Context, height abi.ChainEpoch) (big.Int, error) {
	return c.circulatingSupplyCalculator.GetFilVested(ctx, height)
}
