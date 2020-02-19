package vm

import (
	"bytes"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
)

// FakeVMContext creates the scaffold for faking out the vm context for direct calls to actors
type FakeVMContext struct {
	MessageValue            *types.UnsignedMessage
	StorageValue            specsruntime.Store
	BalanceValue            abi.TokenAmount
	BlockHeightValue        abi.ChainEpoch
	VerifierValue           verification.Verifier
	BlockMinerValue         address.Address
	RandomnessValue         []byte
	IsFromAccountActorValue bool
	Sender                  func(address.Address, abi.MethodNum, specsruntime.CBORMarshaler, abi.TokenAmount) (specsruntime.SendReturn, exitcode.ExitCode)
	Addresser               func() (address.Address, error)
	Charger                 func(cost types.GasUnits) error
	Sampler                 func(sampleHeight *types.BlockHeight) ([]byte, error)
	ActorCreator            func(addr address.Address, code cid.Cid) error
	allowSideEffects        bool
	stateHandle             specsruntime.StateHandle
}

// NewFakeVMContext fakes the state machine infrastructure so actor methods can be called directly
func NewFakeVMContext(message *types.UnsignedMessage, state interface{}) *FakeVMContext {
	randomness := make([]byte, 32)
	copy(randomness[:], []byte("only random in the figurative sense"))

	addressGetter := vmaddr.NewForTestGetter()
	store := &TestStorage{state: state}
	aux := FakeVMContext{
		MessageValue:            message,
		StorageValue:            store,
		BlockHeightValue:        0,
		BalanceValue:            big.Zero(),
		RandomnessValue:         randomness,
		IsFromAccountActorValue: true,
		BlockMinerValue:         address.Undef,
		Charger: func(cost types.GasUnits) error {
			return nil
		},
		Sampler: func(sampleHeight *types.BlockHeight) ([]byte, error) {
			return randomness, nil
		},
		Sender: func(to address.Address, method abi.MethodNum, params specsruntime.CBORMarshaler, value abi.TokenAmount) (specsruntime.SendReturn, exitcode.ExitCode) {
			return nil, exitcode.Ok
		},
		Addresser: func() (address.Address, error) {
			return addressGetter(), nil
		},
		ActorCreator: func(addr address.Address, code cid.Cid) error {
			return nil
		},
		allowSideEffects: true,
	}
	aux.stateHandle = vmcontext.NewActorStateHandle(&aux, store.CidOf(state))
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
func (tc *FakeVMContext) CurrentEpoch() abi.ChainEpoch {
	return tc.BlockHeightValue
}

// Randomness provides random bytes used in verification challenges
func (tc *FakeVMContext) Randomness(epoch abi.ChainEpoch) abi.Randomness {
	rnd, _ := tc.Sampler(types.NewBlockHeight((uint64)(epoch)))
	return rnd
}

// Send allows actors to invoke methods on other actors
func (tc *FakeVMContext) Send(toAddr address.Address, methodNum abi.MethodNum, params specsruntime.CBORMarshaler, value abi.TokenAmount) (specsruntime.SendReturn, exitcode.ExitCode) {
	// check if side-effects are allowed
	if !tc.allowSideEffects {
		runtime.Abortf(exitcode.SysErrInternal, "Calling Send() is not allowed during side-effet lock")
	}
	return tc.Sender(toAddr, methodNum, params, value)
}

var _ specsruntime.Message = &FakeVMContext{}

// BlockMiner is the address for the actor miner who mined the block in which the initial on-chain message appears.
func (tc *FakeVMContext) BlockMiner() address.Address {
	return tc.BlockMinerValue
}

// Caller is the immediate caller to the current executing method.
func (tc *FakeVMContext) Caller() address.Address {
	return tc.MessageValue.From
}

// Receiver implements Message.
func (tc *FakeVMContext) Receiver() address.Address {
	return tc.MessageValue.To
}

// ValueReceived is the amount of FIL received by this actor during this method call.
//
// Note: the value is already been deposited on the actors account and is reflected on the balance.
func (tc *FakeVMContext) ValueReceived() abi.TokenAmount {
	// Dragons: temp until we remove legacy types
	return tc.MessageValue.Value
}

var _ runtime.InvocationContext = &FakeVMContext{}

// Runtime exposes some methods on the runtime to the actor.
func (tc *FakeVMContext) Runtime() runtime.Runtime {
	return tc
}

// Message implements the interface.
func (tc *FakeVMContext) Message() specsruntime.Message {
	return tc
}

// ValidateCaller validates the caller against a patter.
//
// All actor methods MUST call this method before returning.
func (tc *FakeVMContext) ValidateCaller(pattern runtime.CallerPattern) {
	// Note: the FakeVMContext is currently harcoded to a single pattern
	if !tc.IsFromAccountActorValue {
		runtime.Abortf(exitcode.SysErrInternal, "Method invoked by incorrect caller")
	}
}

// State handles access to the actor state.
func (tc *FakeVMContext) State() specsruntime.StateHandle {
	return tc.stateHandle
}

// Balance is the current balance on the current actors account.
//
// Note: the value received for this invocation is already reflected on the balance.
func (tc *FakeVMContext) Balance() abi.TokenAmount {
	return tc.BalanceValue
}

// Store provides and interface to actor state
func (tc *FakeVMContext) Store() specsruntime.Store {
	return tc.StorageValue
}

// Charge charges gas for the current action
func (tc *FakeVMContext) Charge(cost types.GasUnits) error {
	return tc.Charger(cost)
}

var _ runtime.ExtendedInvocationContext = (*FakeVMContext)(nil)

// CreateActor implemenets the ExtendedInvocationContext interface.
func (tc *FakeVMContext) CreateActor(code cid.Cid, addr address.Address) {
	err := tc.ActorCreator(addr, code)
	if err != nil {
		runtime.Abortf(exitcode.SysErrInternal, "Could not create actor")
	}
}

// VerifySignature implemenets the ExtendedInvocationContext interface.
func (*FakeVMContext) VerifySignature(signer address.Address, signature crypto.Signature, msg []byte) bool {
	return crypto.IsValidSignature(msg, signer, signature)
}

// AllowSideEffects determines wether or not the actor code is allowed to produce side-effects.
//
// At this time, any `Send` to the same or another actor is considered a side-effect.
func (tc *FakeVMContext) AllowSideEffects(allow bool) {
	tc.allowSideEffects = allow
}

// TestStorage is a fake storage used for testing.
type TestStorage struct {
	state interface{}
}

// NewTestStorage returns a new "TestStorage"
func NewTestStorage(state interface{}) *TestStorage {
	return &TestStorage{
		state: state,
	}
}

var _ specsruntime.Store = (*TestStorage)(nil)

// Put implements runtime.Store.
func (ts *TestStorage) Put(v specsruntime.CBORMarshaler) cid.Cid {
	ts.state = v
	if cm, ok := v.(cbg.CBORMarshaler); ok {
		buf := new(bytes.Buffer)
		err := cm.MarshalCBOR(buf)
		if err == nil {
			return cid.NewCidV1(cid.Raw, buf.Bytes())
		}
	}
	raw, err := encoding.Encode(v)
	if err != nil {
		panic("failed to encode")
	}
	return cid.NewCidV1(cid.Raw, raw)
}

// Get implements runtime.Store.
func (ts *TestStorage) Get(cid cid.Cid, obj specsruntime.CBORUnmarshaler) bool {
	node, err := cbor.WrapObject(ts.state, types.DefaultHashFunction, -1)
	if err != nil {
		return false
	}

	err = encoding.Decode(node.RawData(), obj)
	if err != nil {
		return false
	}

	return true
}

// CidOf returns the cid of the object.
func (ts *TestStorage) CidOf(obj interface{}) cid.Cid {
	if obj == nil {
		return cid.Undef
	}
	raw, err := encoding.Encode(obj)
	if err != nil {
		panic("failed to encode")
	}
	return cid.NewCidV1(cid.Raw, raw)
}
