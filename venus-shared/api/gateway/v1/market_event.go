package gateway

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-storage/storage"
	types2 "github.com/ipfs-force-community/venus-common-utils/types"
	"github.com/ipfs/go-cid"

	gtypes "github.com/filecoin-project/venus/venus-shared/types/gateway"
)

type IMarketEvent interface {
	ListMarketConnectionsState(ctx context.Context) ([]gtypes.MarketConnectionState, error)                                                                                                //perm:admin
	IsUnsealed(ctx context.Context, miner address.Address, pieceCid cid.Cid, sector storage.SectorRef, offset types2.PaddedByteIndex, size abi.PaddedPieceSize) (bool, error)              //perm:admin
	SectorsUnsealPiece(ctx context.Context, miner address.Address, pieceCid cid.Cid, sector storage.SectorRef, offset types2.PaddedByteIndex, size abi.PaddedPieceSize, dest string) error //perm:admin

	ResponseMarketEvent(ctx context.Context, resp *gtypes.ResponseEvent) error                                       //perm:read
	ListenMarketEvent(ctx context.Context, policy *gtypes.MarketRegisterPolicy) (<-chan *gtypes.RequestEvent, error) //perm:read
}
