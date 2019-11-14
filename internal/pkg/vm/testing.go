package vm

import (
	cbor "github.com/ipfs/go-ipld-cbor"

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
	RandomnessValue         []byte
	IsFromAccountActorValue bool
	Sender                  func(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error)
	Addresser               func() (address.Address, error)
	Charger                 func(cost types.GasUnits) error
	Sampler                 func(sampleHeight *types.BlockHeight) ([]byte, error)
	ActorCreator            func(addr address.Address, code cid.Cid, initalizationParams interface{}) error
}

// NewFakeVMContext fakes the state machine infrastructure so actor methods can be called directly
func NewFakeVMContext(message *types.UnsignedMessage, state interface{}) *FakeVMContext {
	randomness := make([]byte, 32)
	copy(randomness[:], []byte("only random in the figurative sense"))

	addressGetter := address.NewForTestGetter()
	return &FakeVMContext{
		MessageValue:            message,
		StorageValue:            &testStorage{state: state},
		BlockHeightValue:        types.NewBlockHeight(0),
		BalanceValue:            types.ZeroAttoFIL,
		RandomnessValue:         randomness,
		IsFromAccountActorValue: true,
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
		ActorCreator: func(addr address.Address, code cid.Cid, initalizationParams interface{}) error {
			return nil
		},
	}
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
	return tc.Sender(to, method, value, params)
}

var _ runtime.InvocationContext = &FakeVMContext{}

// Runtime exposes some methods on the runtime to the actor.
func (tc *FakeVMContext) Runtime() runtime.Runtime {
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

// Caller is the immediate caller to the current executing method.
func (tc *FakeVMContext) Caller() address.Address {
	return tc.MessageValue.From
}

// StateHandle handles access to the actor state.
func (tc *FakeVMContext) StateHandle() runtime.ActorStateHandle {
	return vmcontext.NewActorStateHandle(tc, tc.StorageValue.Head())
}

// ValueReceived is the amount of FIL received by this actor during this method call.
//
// Note: the value is already been deposited on the actors account and is reflected on the balance.
func (tc *FakeVMContext) ValueReceived() types.AttoFIL {
	return tc.MessageValue.Value
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

// for built-in actors

// CreateNewActor creates an actor of a given type
func (tc *FakeVMContext) CreateNewActor(addr address.Address, code cid.Cid, initalizationParams interface{}) error {
	return tc.ActorCreator(addr, code, initalizationParams)
}

// Verifier provides an interface to the proofs verifier
func (tc *FakeVMContext) Verifier() verification.Verifier {
	return tc.VerifierValue
}

// Dragons: should I stay or should I go?

// Message is the message that triggered this invocation
func (tc *FakeVMContext) Message() *types.UnsignedMessage {
	return tc.MessageValue
}

// AddressForNewActor creates an address to be used to create a new actor
func (tc *FakeVMContext) AddressForNewActor() (address.Address, error) {
	return tc.Addresser()
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

// Commit satisfies the Storage interface but does nothing
func (ts testStorage) Commit(cid.Cid, cid.Cid) error {
	return nil
}

// Head returns an empty Cid to satisfy the Storage interface
func (ts testStorage) Head() cid.Cid {
	raw, err := encoding.Encode(ts.state)
	if err != nil {
		panic("failed to encode")
	}
	return cid.NewCidV1(cid.Raw, raw)
}
