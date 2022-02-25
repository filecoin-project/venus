package types

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/internal"
	"github.com/ipfs/go-cid"
)

var ErrActorNotFound = internal.ErrActorNotFound

type Actor = internal.Actor

// NewActor constructs a new actor.
func NewActor(code cid.Cid, balance abi.TokenAmount, head cid.Cid) *Actor {
	return &Actor{
		Code:    code,
		Nonce:   0,
		Balance: balance,
		Head:    head,
	}
}
