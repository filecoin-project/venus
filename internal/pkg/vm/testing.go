package vm

import (
	"bytes"

	cbor "github.com/ipfs/go-ipld-cbor"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
	"github.com/ipfs/go-cid"
)

// FakeVMContext creates the scaffold for faking out the vm context for direct calls to actors
type FakeVMContext struct {
	MessageValue            *types.UnsignedMessage
	StorageValue            runtime.Storage
	BalanceValue            types.AttoFIL
	BlockHeightValue        *types.BlockHeight
	VerifierValue           verification.Verifier
	BlockMinerValue         address.Address
	RandomnessValue         []byte
	IsFromAccountActorValue bool
	Sender                  func(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error)
	Addresser               func() (address.Address, error)
	Charger                 func(cost types.GasUnits) error
	Sampler                 func(sampleHeight *types.BlockHeight) ([]byte, error)
	ActorCreator            func(addr address.Address, code cid.Cid) error
	allowSideEffects        bool
	stateHandle             runtime.ActorStateHandle
}

// NewFakeVMContext fakes the state machine infrastructure so actor methods can be called directly
func NewFakeVMContext(message *types.UnsignedMessage, state interface{}) *FakeVMContext {
	randomness := make([]byte, 32)
	copy(randomness[:], []byte("only random in the figurative sense"))

	addressGetter := address.NewForTestGetter()
	aux := FakeVMContext{
		MessageValue:            message,
		StorageValue:            &testStorage{state: state},
		BlockHeightValue:        types.NewBlockHeight(0),
		BalanceValue:            types.ZeroAttoFIL,
		RandomnessValue:         randomness,
		IsFromAccountActorValue: true,
		BlockMinerValue:         address.Undef,
		Charger: func(cost types.GasUnits) error {
			return nil
		},
		Sampler: func(sampleHeight *types.BlockHeight) ([]byte, error) {
			return randomness, nil
		},
		Sender: func(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
			return [][]byte{}, 0, nil
		},
		Addresser: func() (address.Address, error) {
			return addressGetter(), nil
		},
		ActorCreator: func(addr address.Address, code cid.Cid) error {
			return nil
		},
		allowSideEffects: true,
	}
	aux.stateHandle = vmcontext.NewActorStateHandle(&aux, aux.StorageValue.Head())
	return &aux
}

// NewFakeVMContextWithVerifier creates a fake VMContext with the given verifier
func NewFakeVMContextWithVerifier(message *types.UnsignedMessage, state interface{}, verifier verification.Verifier) *FakeVMContext {
	vmctx := NewFakeVMContext(message, state)
	vmctx.VerifierValue = verifier
	return vmctx
}

var _ runtime.Runtime = &FakeVMContext{}

// CurrentEpoch is the current chain height
func (tc *FakeVMContext) CurrentEpoch() types.BlockHeight {
	return *tc.BlockHeightValue
}

// Randomness provides random bytes used in verification challenges
func (tc *FakeVMContext) Randomness(epoch types.BlockHeight, offset uint64) runtime.Randomness {
	rnd, _ := tc.Sampler(&epoch)
	return rnd
}

// Send sends a message to another actor
func (tc *FakeVMContext) Send(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
	// check if side-effects are allowed
	if !tc.allowSideEffects {
		runtime.Abort("Calling Send() is not allowed during side-effet lock")
	}
	return tc.Sender(to, method, value, params)
}

var _ runtime.MessageInfo = &FakeVMContext{}

// BlockMiner is the address for the actor miner who mined the block in which the initial on-chain message appears.
func (tc *FakeVMContext) BlockMiner() address.Address {
	return tc.BlockMinerValue
}

// Caller is the immediate caller to the current executing method.
func (tc *FakeVMContext) Caller() address.Address {
	return tc.MessageValue.From
}

// ValueReceived is the amount of FIL received by this actor during this method call.
//
// Note: the value is already been deposited on the actors account and is reflected on the balance.
func (tc *FakeVMContext) ValueReceived() types.AttoFIL {
	return tc.MessageValue.Value
}

var _ runtime.InvocationContext = &FakeVMContext{}

// Runtime exposes some methods on the runtime to the actor.
func (tc *FakeVMContext) Runtime() runtime.Runtime {
	return tc
}

// Message implements the interface.
func (tc *FakeVMContext) Message() runtime.MessageInfo {
	return tc
}

// ValidateCaller validates the caller against a patter.
//
// All actor methods MUST call this method before returning.
func (tc *FakeVMContext) ValidateCaller(pattern runtime.CallerPattern) {
	// Note: the FakeVMContext is currently harcoded to a single pattern
	if !tc.IsFromAccountActorValue {
		runtime.Abort("Method invoked by incorrect caller")
	}
}

// StateHandle handles access to the actor state.
func (tc *FakeVMContext) StateHandle() runtime.ActorStateHandle {
	return tc.stateHandle
}

// Balance is the current balance on the current actors account.
//
// Note: the value received for this invocation is already reflected on the balance.
func (tc *FakeVMContext) Balance() types.AttoFIL {
	return tc.BalanceValue
}

// Storage provides and interface to actor state
func (tc *FakeVMContext) Storage() runtime.Storage {
	return tc.StorageValue
}

// Charge charges gas for the current action
func (tc *FakeVMContext) Charge(cost types.GasUnits) error {
	return tc.Charger(cost)
}

var _ runtime.ExtendedInvocationContext = (*FakeVMContext)(nil)

var _ runtime.LegacyInvocationContext = (*FakeVMContext)(nil)

// LegacyCreateNewActor creates an actor of a given type
func (tc *FakeVMContext) LegacyCreateNewActor(addr address.Address, code cid.Cid) error {
	return tc.ActorCreator(addr, code)
}

// LegacyVerifier provides an interface to the proofs verifier
func (tc *FakeVMContext) LegacyVerifier() verification.Verifier {
	return tc.VerifierValue
}

// LegacyMessage is the message that triggered this invocation
func (tc *FakeVMContext) LegacyMessage() *types.UnsignedMessage {
	return tc.MessageValue
}

// LegacyAddressForNewActor creates an address to be used to create a new actor
func (tc *FakeVMContext) LegacyAddressForNewActor() (address.Address, error) {
	return tc.Addresser()
}

// AllowSideEffects determines wether or not the actor code is allowed to produce side-effects.
//
// At this time, any `Send` to the same or another actor is considered a side-effect.
func (tc *FakeVMContext) AllowSideEffects(allow bool) {
	tc.allowSideEffects = allow
}

type testStorage struct {
	state interface{}
}

// NewTestStorage returns a new "testStorage"
func NewTestStorage(state interface{}) runtime.Storage {
	return &testStorage{
		state: state,
	}
}

var _ runtime.Storage = &testStorage{}

// Put satisfies the Storage interface
func (ts *testStorage) Put(v interface{}) (cid.Cid, error) {
	ts.state = v
	if cm, ok := v.(cbg.CBORMarshaler); ok {
		buf := new(bytes.Buffer)
		err := cm.MarshalCBOR(buf)
		if err == nil {
			nd, err := cbor.Decode(buf.Bytes(), types.DefaultHashFunction, -1)
			if err != nil {
				panic("failed to decode")
			}
			return nd.Cid(), err
		}
	}
	raw, err := encoding.Encode(v)
	if err != nil {
		panic("failed to encode")
	}
	return cid.NewCidV1(cid.Raw, raw), nil
}

// Get returns the internal state variable encoded into bytes
func (ts testStorage) Get(cid.Cid) ([]byte, error) {
	node, err := cbor.WrapObject(ts.state, types.DefaultHashFunction, -1)
	if err != nil {
		return []byte{}, err
	}

	return node.RawData(), nil
}

// CidOf returns the Cid of the object.
func (ts testStorage) CidOf(v interface{}) (cid.Cid, error) {
	if v == nil {
		return cid.Undef, nil
	}
	raw, err := encoding.Encode(v)
	if err != nil {
		return cid.Undef, err
	}
	return cid.NewCidV1(cid.Raw, raw), nil
}

// Commit satisfies the Storage interface but does nothing
func (ts testStorage) Commit(cid.Cid, cid.Cid) error {
	return nil
}

// Head returns the Cid of the current state.
func (ts testStorage) Head() cid.Cid {
	if ts.state == nil {
		return cid.Undef
	}
	raw, err := encoding.Encode(ts.state)
	if err != nil {
		panic("failed to encode")
	}
	return cid.NewCidV1(cid.Raw, raw)
}
