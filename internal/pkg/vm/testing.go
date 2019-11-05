package vm

import (
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
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

var _ runtime.Runtime = &FakeVMContext{}

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

// Message is the message that triggered this invocation
func (tc *FakeVMContext) Message() *types.UnsignedMessage {
	return tc.MessageValue
}

// Storage provides and interface to actor state
func (tc *FakeVMContext) Storage() runtime.Storage {
	return tc.StorageValue
}

// Send sends a message to another actor
func (tc *FakeVMContext) Send(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
	return tc.Sender(to, method, value, params)
}

// AddressForNewActor creates an address to be used to create a new actor
func (tc *FakeVMContext) AddressForNewActor() (address.Address, error) {
	return tc.Addresser()
}

// BlockHeight is the current chain height
func (tc *FakeVMContext) BlockHeight() *types.BlockHeight {
	return tc.BlockHeightValue
}

// MyBalance is the balance of the current actor
func (tc *FakeVMContext) MyBalance() types.AttoFIL {
	return tc.BalanceValue
}

// IsFromAccountActor returns true if the actor that sent the message is an account actor
func (tc *FakeVMContext) IsFromAccountActor() bool {
	return tc.IsFromAccountActorValue
}

// Charge charges gas for the current action
func (tc *FakeVMContext) Charge(cost types.GasUnits) error {
	return tc.Charger(cost)
}

// SampleChainRandomness provides random bytes used in verification challenges
func (tc *FakeVMContext) SampleChainRandomness(sampleHeight *types.BlockHeight) ([]byte, error) {
	return tc.Sampler(sampleHeight)
}

// CreateNewActor creates an actor of a given type
func (tc *FakeVMContext) CreateNewActor(addr address.Address, code cid.Cid, initalizationParams interface{}) error {
	return tc.ActorCreator(addr, code, initalizationParams)
}

// Verifier provides an interface to the proofs verifier
func (tc *FakeVMContext) Verifier() verification.Verifier {
	return tc.VerifierValue
}

type testStorage struct {
	state interface{}
}

var _ runtime.Storage = &testStorage{}

// Put satisfies the Storage interface
func (ts *testStorage) Put(v interface{}) (cid.Cid, error) {
	ts.state = v
	return cid.Cid{}, nil
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
	return cid.Cid{}
}
