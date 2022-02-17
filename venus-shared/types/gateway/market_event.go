package gateway

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-storage/storage"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

type MarketRegisterPolicy struct {
	Miner address.Address
}

type IsUnsealRequest struct {
	PieceCid cid.Cid
	Sector   storage.SectorRef
	Offset   types.PaddedByteIndex
	Size     abi.PaddedPieceSize
}

type IsUnsealResponse struct {
}

type UnsealRequest struct {
	PieceCid cid.Cid
	Sector   storage.SectorRef
	Offset   types.PaddedByteIndex
	Size     abi.PaddedPieceSize
	Dest     string
}

type UnsealResponse struct {
}

type MarketConnectionState struct {
	Addr address.Address
	Conn ConnectionStates
}
