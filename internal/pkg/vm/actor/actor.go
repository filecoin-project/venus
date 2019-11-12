// Package actor implements tooling to write and manipulate actors in go.
package actor

import (
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// DefaultGasCost is default gas cost for the actor calls.
const DefaultGasCost = 100

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
	Code cid.Cid `refmt:",omitempty"`
	// Head is the CID of the root of the actor's state tree.
	Head cid.Cid `refmt:",omitempty"`
	// Nonce is the nonce expected on the next message from this actor.
	// Messages are processed in strict, contiguous nonce order.
	Nonce types.Uint64
	// Balance is the amount of FIL in the actor's account.
	Balance types.AttoFIL
}

// NewActor constructs a new actor.
func NewActor(code cid.Cid, balance types.AttoFIL) *Actor {
	return &Actor{
		Code:    code,
		Head:    cid.Undef,
		Nonce:   0,
		Balance: balance,
	}
}

// Empty tests whether the actor's code is defined.
func (a *Actor) Empty() bool {
	return !a.Code.Defined()
}

// IncNonce increments the nonce of this actor by 1.
func (a *Actor) IncNonce() {
	a.Nonce = a.Nonce + 1
}

// Cid returns the canonical CID for the actor.
// TODO: can we avoid returning an error?
func (a *Actor) Cid() (cid.Cid, error) {
	obj, err := cbor.WrapObject(a, types.DefaultHashFunction, -1)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to marshal to cbor")
	}

	return obj.Cid(), nil
}

// Unmarshal a actor from the given bytes.
func (a *Actor) Unmarshal(b []byte) error {
	return encoding.Decode(b, a)
}

// Marshal the actor into bytes.
func (a *Actor) Marshal() ([]byte, error) {
	return encoding.Encode(a)
}

// Format implements fmt.Formatter.
func (a *Actor) Format(f fmt.State, c rune) {
	f.Write([]byte(fmt.Sprintf("<%s (%p); balance: %v; nonce: %d>", types.ActorCodeTypeName(a.Code), a, a.Balance, a.Nonce))) // nolint: errcheck
}

///// Utility functions (non-methods) /////

// NextNonce returns the nonce value for an account actor, which is the nonce expected on the
// next message to be sent from that actor.
// Returns zero for a nil actor, which is the value expected on the first message.
func NextNonce(actor *Actor) (uint64, error) {
	if actor == nil {
		return 0, nil
	}
	if !(actor.Empty() || actor.Code.Equals(types.AccountActorCodeCid)) {
		return 0, errors.New("next nonce only defined for account or empty actors")
	}
	return uint64(actor.Nonce), nil
}

// InitBuiltinActorCodeObjs writes all builtin actor code objects to `cst`. This method should be called when initializing a genesis
// block to ensure all IPLD links referenced by the state tree exist.
func InitBuiltinActorCodeObjs(cst *hamt.CborIpldStore) error {
	if err := cst.Blocks.AddBlock(types.StorageMarketActorCodeObj); err != nil {
		return err
	}
	if err := cst.Blocks.AddBlock(types.MinerActorCodeObj); err != nil {
		return err
	}
	if err := cst.Blocks.AddBlock(types.BootstrapMinerActorCodeObj); err != nil {
		return err
	}
	if err := cst.Blocks.AddBlock(types.AccountActorCodeObj); err != nil {
		return err
	}
	if err := cst.Blocks.AddBlock(types.PaymentBrokerActorCodeObj); err != nil {
		return err
	}
	if err := cst.Blocks.AddBlock(types.PowerActorCodeObj); err != nil {
		return err
	}

	return cst.Blocks.AddBlock(types.InitActorCodeObj)

}
