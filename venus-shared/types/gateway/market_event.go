package gateway

import (
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type MarketRegisterPolicy struct {
	Miner address.Address
}

type UnsealRequest struct {
	PieceCid cid.Cid
	Miner    address.Address
	Sid      abi.SectorNumber
	Offset   types.UnpaddedByteIndex
	Size     abi.UnpaddedPieceSize
	Dest     string
}

type UnsealState string

const (
	UnsealStateSet       UnsealState = "set_up"
	UnsealStateFinished  UnsealState = "finished"
	UnsealStateFailed    UnsealState = "failed"
	UnsealStateUnsealing UnsealState = "unsealing"
	UnsealStateUploading UnsealState = "uploading"
)

type MarketConnectionState struct {
	Addr address.Address
	Conn ConnectionStates
}
