package gateway

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-storage/storage"
	"github.com/filecoin-project/venus/venus-shared/types"
	gtypes "github.com/filecoin-project/venus/venus-shared/types/gateway"
	"github.com/ipfs/go-cid"
)

type IMarketEvent interface {
	IMarketClient
	IMarketServiceProvider
}

type IMarketClient interface {
	ListMarketConnectionsState(ctx context.Context) ([]gtypes.MarketConnectionState, error)                                                                                               //perm:admin
	IsUnsealed(ctx context.Context, miner address.Address, pieceCid cid.Cid, sector storage.SectorRef, offset types.PaddedByteIndex, size abi.PaddedPieceSize) (bool, error)              //perm:admin
	SectorsUnsealPiece(ctx context.Context, miner address.Address, pieceCid cid.Cid, sector storage.SectorRef, offset types.PaddedByteIndex, size abi.PaddedPieceSize, dest string) error //perm:admin
}

type IMarketServiceProvider interface {
	ResponseMarketEvent(ctx context.Context, resp *gtypes.ResponseEvent) error                                       //perm:read
	ListenMarketEvent(ctx context.Context, policy *gtypes.MarketRegisterPolicy) (<-chan *gtypes.RequestEvent, error) //perm:read
}
