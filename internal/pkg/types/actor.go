// Package actor implements tooling to write and manipulate actors in go.
package types

import (
	"io"
	"io/ioutil"

	fxamackercbor "github.com/fxamacker/cbor/v2"
	"github.com/ipfs/go-cid"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/internal/pkg/enccid"
)

var ErrActorNotFound = errors.New("actor not found")

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
	_ struct{} `cbor:",toarray"`
	// Code is a CID of the VM code for this actor's implementation (or a constant for actors implemented in Go code).
	// Code may be nil for an uninitialized actor (which exists because it has received a balance).
	Code enccid.Cid
	// Head is the CID of the root of the actor's state tree.
	Head enccid.Cid
	// CallSeqNum is the number expected on the next message from this actor.
	// Messages are processed in strict, contiguous order.
	CallSeqNum uint64
	// Balance is the amount of attoFIL in the actor's account.
	Balance abi.TokenAmount
}

// NewActor constructs a new actor.
func NewActor(code cid.Cid, balance abi.TokenAmount, head cid.Cid) *Actor {
	return &Actor{
		Code:       enccid.NewCid(code),
		CallSeqNum: 0,
		Balance:    balance,
		Head:       enccid.NewCid(head),
	}
}

// Empty tests whether the actor's code is defined.
func (a *Actor) Empty() bool {
	return !a.Code.Defined()
}

// IncrementSeqNum increments the seq number.
func (a *Actor) IncrementSeqNum() {
	a.CallSeqNum = a.CallSeqNum + 1
}

// UnmarshalCBOR must implement cbg.Unmarshaller to insert this into a hamt.
func (a *Actor) UnmarshalCBOR(r io.Reader) error {
	bs, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return fxamackercbor.Unmarshal(bs, a)
}

// MarshalCBOR must implement cbg.Marshaller to insert this into a hamt.
func (a *Actor) MarshalCBOR(w io.Writer) error {
	bs, err := fxamackercbor.Marshal(a)
	if err != nil {
		return err
	}
	_, err = w.Write(bs)
	return err
}
