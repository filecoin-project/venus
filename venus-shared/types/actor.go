package types

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/internal"
	"github.com/ipfs/go-cid"
)

var ErrActorNotFound = internal.ErrActorNotFound

type ActorV4 = internal.ActorV4
type Actor = internal.Actor // actorV5

// NewActor constructs a new actor.
func NewActor(code cid.Cid, balance abi.TokenAmount, head cid.Cid, addr address.Address) *Actor {
	return &Actor{
		Code:    code,
		Nonce:   0,
		Balance: balance,
		Head:    head,
		Address: &addr,
	}
}

var (
	AsActorV4 = internal.AsActorV4
	AsActorV5 = internal.AsActorV5
)
