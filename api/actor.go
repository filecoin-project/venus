package api

import (
	"context"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

type ActorView struct {
	ActorType string          `json:"actorType"`
	Address   string          `json:"address"`
	Code      *cid.Cid        `json:"code"`
	Nonce     uint64          `json:"nonce"`
	Balance   *types.AttoFIL  `json:"balance"`
	Exports   ReadableExports `json:"exports"`
	Head      *cid.Cid        `json:"head"`
}

type ReadableFunctionSignature struct {
	Params []string
	Return []string
}
type ReadableExports map[string]*ReadableFunctionSignature

type ActorAPI interface {
	Ls(ctx context.Context) ([]*ActorView, error)
}
