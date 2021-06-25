package types

import (
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-state-types/abi"
)

var ErrActorNotFound = errors.New("actor not found")

// Actor is the central abstraction of entities in the system.
//
// Both individual accounts, as well as contracts (user & system level) are
// represented as actors. An actor has the following core functionality implemented on a system level:
// - track a Filecoin balance, using the `Balance` field
// - execute code stored in the `Code` field
// - read & write memory
// - replay protection, using the `Nonce` field
//
// Value sent to a non-existent address will be tracked as an empty actor that has a Balance but
// nil Code and Memory. You must nil check Code cids before comparing them.
//
// More specific capabilities for individual accounts or contract specific must be implemented
// inside the code.
//
// Not safe for concurrent access.
type Actor struct {
	// Code is a CID of the VM code for this actor's implementation (or a constant for actors implemented in Go code).
	// Code may be nil for an uninitialized actor (which exists because it has received a balance).
	Code cid.Cid
	// Head is the CID of the root of the actor's state tree.
	Head cid.Cid
	// Nonce is the number expected on the next message from this actor.
	// Messages are processed in strict, contiguous order.
	Nonce uint64
	// Balance is the amount of attoFIL in the actor's account.
	Balance abi.TokenAmount
}

// NewActor constructs a new actor.
func NewActor(code cid.Cid, balance abi.TokenAmount, head cid.Cid) *Actor {
	return &Actor{
		Code:    code,
		Nonce:   0,
		Balance: balance,
		Head:    head,
	}
}

// Empty tests whether the actor's code is defined.
func (t *Actor) Empty() bool {
	return !t.Code.Defined()
}

// IncrementSeqNum increments the seq number.
func (t *Actor) IncrementSeqNum() {
	t.Nonce = t.Nonce + 1
}
