package api

import (
	"context"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"

	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
)

type Message interface {
	Send(ctx context.Context, from, to types.Address, val *types.AttoFIL) (*cid.Cid, error)
	Wait(ctx context.Context, msgCid *cid.Cid, cb func(blk *types.Block, msg *types.SignedMessage, receipt *types.MessageReceipt, signature *exec.FunctionSignature) error) error
}
