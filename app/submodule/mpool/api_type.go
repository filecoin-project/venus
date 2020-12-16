package mpool

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/types"
)

type MsgLookup struct {
	Message   cid.Cid // Can be different than requested, in case it was replaced, but only gas values changed
	Receipt   types.MessageReceipt
	ReturnDec interface{}
	TipSet    block.TipSetKey
	Height    abi.ChainEpoch
}
