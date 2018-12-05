package api

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

// ActorView represents a generic way to represent details about any actor to the user.
type ActorView struct {
	ActorType string          `json:"actorType"`
	Address   string          `json:"address"`
	Code      cid.Cid         `json:"code,omitempty"`
	Nonce     uint64          `json:"nonce"`
	Balance   *types.AttoFIL  `json:"balance"`
	Exports   ReadableExports `json:"exports"`
	Head      cid.Cid         `json:"head,omitempty"`
}

// ReadableFunctionSignature is a representation of an actors function signature,
// such that it can be shown to the user.
type ReadableFunctionSignature struct {
	Params []string
	Return []string
}

// ReadableExports is a representation of exports (map of method names to signatures),
// such that it can be shown to the user.
type ReadableExports map[string]*ReadableFunctionSignature

// Actor is the interface that defines methods to inspect actors, which are Filecoin's
// notion of smart contracts.
type Actor interface {
	Ls(ctx context.Context) ([]*ActorView, error)
}
