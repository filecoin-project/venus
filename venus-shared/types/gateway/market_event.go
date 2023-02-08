package gateway

import (
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type MarketRegisterPolicy struct {
	Miner address.Address
}

type IsUnsealRequest struct {
	PieceCid cid.Cid
	Miner    address.Address
	Sid      abi.SectorNumber
	Offset   types.PaddedByteIndex
	Size     abi.PaddedPieceSize
}

type IsUnsealResponse struct{}

type UnsealRequest struct {
	PieceCid cid.Cid
	Miner    address.Address
	Sid      abi.SectorNumber
	Offset   types.PaddedByteIndex
	Size     abi.PaddedPieceSize
	Transfer market.Transfer
}

type UnsealResponse struct{}

type MarketConnectionState struct {
	Addr address.Address
	Conn ConnectionStates
}
