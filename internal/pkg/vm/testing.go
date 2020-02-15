package vm

import (
	"bytes"

	"github.com/filecoin-project/go-address"
	cbor "github.com/ipfs/go-ipld-cbor"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
)

// FakeVMContext creates the scaffold for faking out the vm context for direct calls to actors
type FakeVMContext struct {
	MessageValue            *types.UnsignedMessage
	StorageValue            runtime.Storage
	BalanceValue            abi.TokenAmount
	BlockHeightValue        abi.ChainEpoch
	VerifierValue           verification.Verifier
	BlockMinerValue         address.Address
	RandomnessValue         []byte
	IsFromAccountActorValue bool
	LegacySender            func(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error)
	Sender                  func(to address.Address, method types.MethodID, value abi.TokenAmount, params interface{}) interface{}
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

	addressGetter := vmaddr.NewForTestGetter()
	aux := FakeVMContext{
		MessageValue:            message,
		StorageValue:            &testStorage{state: state},
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
		LegacySender: func(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
			return [][]byte{}, 0, nil
		},
		Sender: func(to address.Address, method types.MethodID, value abi.TokenAmount, params interface{}) interface{} {
			return nil
		},
		Addresser: func() (address.Address, error) {
			return addressGetter(), nil
		},
		ActorCreator: func(addr address.Address, code cid.Cid) error {
			return nil
		},
		allowSideEffects: true,
	}
	aux.stateHandle = vmcontext.NewActorStateHandle(&aux, aux.StorageValue.CidOf(state))
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

// LegacySend sends a message to another actor
func (tc *FakeVMContext) LegacySend(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
	// check if side-effects are allowed
	if !tc.allowSideEffects {
		runtime.Abortf(exitcode.SysErrInternal, "Calling LegacySend() is not allowed during side-effet lock")
	}
	return tc.LegacySender(to, method, value, params)
}

// Send allows actors to invoke methods on other actors
func (tc *FakeVMContext) Send(to address.Address, method types.MethodID, value abi.TokenAmount, params interface{}) interface{} {
	// check if side-effects are allowed
	if !tc.allowSideEffects {
		runtime.Abortf(exitcode.SysErrInternal, "Calling Send() is not allowed during side-effet lock")
	}
	return tc.Sender(to, method, value, params)
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
	return abi.NewTokenAmount(tc.MessageValue.Value.AsBigInt().Int64())
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
func (tc *FakeVMContext) State() runtime.ActorStateHandle {
	return tc.stateHandle
}

// Balance is the current balance on the current actors account.
//
// Note: the value received for this invocation is already reflected on the balance.
func (tc *FakeVMContext) Balance() abi.TokenAmount {
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

// CreateActor implemenets the ExtendedInvocationContext interface.
func (tc *FakeVMContext) CreateActor(actorID types.Uint64, code cid.Cid, params []byte) (address.Address, address.Address) {
	addr, err := tc.Addresser()
	if err != nil {
		runtime.Abortf(exitcode.SysErrInternal, "Could not create address")
	}
	idAddr, err := address.NewIDAddress(uint64(actorID))
	if err != nil {
		runtime.Abortf(exitcode.SysErrInternal, "Could not create IDAddress for actor")
	}
	err = tc.ActorCreator(idAddr, code)
	if err != nil {
		runtime.Abortf(exitcode.SysErrInternal, "Could not create actor")
	}

	return idAddr, addr
}

// VerifySignature implemenets the ExtendedInvocationContext interface.
func (*FakeVMContext) VerifySignature(signer address.Address, signature types.Signature, msg []byte) bool {
	return types.IsValidSignature(msg, signer, signature)
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

var _ runtime.Storage = (*testStorage)(nil)

func (ts *testStorage) Put(v interface{}) cid.Cid {
	ts.state = v
	if cm, ok := v.(cbg.CBORMarshaler); ok {
		buf := new(bytes.Buffer)
		err := cm.MarshalCBOR(buf)
		if err == nil {
			nd, err := cbor.Decode(buf.Bytes(), types.DefaultHashFunction, -1)
			if err != nil {
				panic("failed to decode")
			}
			return nd.Cid()
		}
	}
	raw, err := encoding.Encode(v)
	if err != nil {
		panic("failed to encode")
	}
	return cid.NewCidV1(cid.Raw, raw)
}

func (ts *testStorage) Get(cid cid.Cid, obj interface{}) bool {
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

func (ts *testStorage) GetRaw(cid cid.Cid) ([]byte, bool) {
	node, err := cbor.WrapObject(ts.state, types.DefaultHashFunction, -1)
	if err != nil {
		return nil, false
	}

	return node.RawData(), true
}

func (ts *testStorage) CidOf(obj interface{}) cid.Cid {
	if obj == nil {
		return cid.Undef
	}
	raw, err := encoding.Encode(obj)
	if err != nil {
		panic("failed to encode")
	}
	return cid.NewCidV1(cid.Raw, raw)
}
