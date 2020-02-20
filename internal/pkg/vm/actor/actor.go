// Package actor implements tooling to write and manipulate actors in go.
package actor

import (
	"fmt"
	"io"
	"io/ioutil"

	fxamackercbor "github.com/fxamacker/cbor"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/pkg/errors"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"

	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
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
	Code e.Cid
	// Head is the CID of the root of the actor's state tree.
	Head e.Cid
	// CallSeqNum is the number expected on the next message from this actor.
	// Messages are processed in strict, contiguous order.
	CallSeqNum uint64
	// Balance is the amount of FIL in the actor's account.
	Balance abi.TokenAmount
}

// NewActor constructs a new actor.
func NewActor(code cid.Cid, balance abi.TokenAmount) *Actor {
	return &Actor{
		Code:       e.NewCid(code),
		CallSeqNum: 0,
		Balance:    balance,
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

// Cid returns the canonical CID for the actor.
// TODO: can we avoid returning an error?
func (a *Actor) Cid() (cid.Cid, error) {
	bs, err := encoding.Encode(a)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to marshal to cbor")
	}
	obj, err := cbor.Decode(bs, types.DefaultHashFunction, -1)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to cast cbor bytes to ipld node")
	}

	return obj.Cid(), nil
}

// Unmarshal a actor from the given bytes.
func (a *Actor) Unmarshal(b []byte) error {
	return fxamackercbor.Unmarshal(b, a)
}

// Marshal the actor into bytes.
func (a *Actor) Marshal() ([]byte, error) {
	return fxamackercbor.Marshal(a, fxamackercbor.EncOptions{})
}

// UnmarshalCBOR must implement cbg.Unmarshaller to insert this into a hamt.
func (a *Actor) UnmarshalCBOR(r io.Reader) error {
	bs, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return a.Unmarshal(bs)
}

// MarshalCBOR must implement cbg.Marshaller to insert this into a hamt.
func (a *Actor) MarshalCBOR(w io.Writer) error {
	bs, err := a.Marshal()
	if err != nil {
		return err
	}
	_, err = w.Write(bs)
	return err
}

// Format implements fmt.Formatter.
func (a *Actor) Format(f fmt.State, c rune) {
	f.Write([]byte(fmt.Sprintf("<%s (%p); balance: %v; nonce: %d>", NameByCode(a.Code.Cid), a, a.Balance, a.CallSeqNum))) // nolint: errcheck
}

// NextNonce returns the nonce value for an account actor, which is the nonce expected on the
// next message to be sent from that actor.
// Returns zero for a nil actor, which is the value expected on the first message.
func NextNonce(actor *Actor) (uint64, error) {
	if actor == nil {
		return 0, nil
	}
	if !(actor.Empty() || actor.Code.Equals(builtin.AccountActorCodeID)) {
		return 0, errors.New("next nonce only defined for account or empty actors")
	}
	return actor.CallSeqNum, nil
}

// NameByCode returns the (string) name of the Go type of the actor with cid, code.
// Dragons: remove once we get rid of the bootstrap actor
func NameByCode(code cid.Cid) string {
	return builtin.ActorNameByCode(code)
}
