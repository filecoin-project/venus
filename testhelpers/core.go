package testhelpers

import (
	"context"
	"errors"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/stretchr/testify/require"
)

// RequireMakeStateTree takes a map of addresses to actors and stores them on
// the state tree, requiring that all its steps succeed.
func RequireMakeStateTree(t *testing.T, cst *hamt.CborIpldStore, acts map[address.Address]*actor.Actor) (cid.Cid, state.Tree) {
	ctx := context.Background()
	tree := state.NewEmptyStateTreeWithActors(cst, builtin.Actors)

	for addr, act := range acts {
		err := tree.SetActor(ctx, addr, act)
		require.NoError(t, err)
	}

	c, err := tree.Flush(ctx)
	require.NoError(t, err)

	return c, tree
}

// RequireNewEmptyActor creates a new empty actor with the given starting
// value and requires that its steps succeed.
func RequireNewEmptyActor(value types.AttoFIL) *actor.Actor {
	return &actor.Actor{Balance: value}
}

// RequireNewAccountActor creates a new account actor with the given starting
// value and requires that its steps succeed.
func RequireNewAccountActor(t *testing.T, value types.AttoFIL) *actor.Actor {
	act, err := account.NewActor(value)
	require.NoError(t, err)
	return act
}

// RequireNewMinerActor creates a new miner actor with the given owner, pledge, and collateral,
// and requires that its steps succeed.
func RequireNewMinerActor(t *testing.T, vms vm.StorageMap, addr address.Address, owner address.Address, pledge uint64, pid peer.ID, coll types.AttoFIL) *actor.Actor {
	act := actor.NewActor(types.MinerActorCodeCid, coll)
	storage := vms.NewStorage(addr, act)
	initializerData := miner.NewState(owner, owner, pid, types.OneKiBSectorSize)
	err := (&miner.Actor{}).InitializeState(storage, initializerData)
	require.NoError(t, err)
	require.NoError(t, storage.Flush())
	return act
}

// RequireNewFakeActor instantiates and returns a new fake actor and requires
// that its steps succeed.
func RequireNewFakeActor(t *testing.T, vms vm.StorageMap, addr address.Address, codeCid cid.Cid) *actor.Actor {
	return RequireNewFakeActorWithTokens(t, vms, addr, codeCid, types.NewAttoFILFromFIL(100))
}

// RequireNewFakeActorWithTokens instantiates and returns a new fake actor and requires
// that its steps succeed.
func RequireNewFakeActorWithTokens(t *testing.T, vms vm.StorageMap, addr address.Address, codeCid cid.Cid, amt types.AttoFIL) *actor.Actor {
	act := actor.NewActor(codeCid, amt)
	store := vms.NewStorage(addr, act)
	err := (&actor.FakeActor{}).InitializeState(store, &actor.FakeActorStorage{})
	require.NoError(t, err)
	require.NoError(t, vms.Flush())
	return act
}

// RequireRandomPeerID returns a new libp2p peer ID or panics.
func RequireRandomPeerID(t *testing.T) peer.ID {
	pid, err := RandPeerID()
	require.NoError(t, err)
	return pid
}

// MockMessagePoolValidator is a mock validator
type MockMessagePoolValidator struct {
	Valid bool
}

// NewMockMessagePoolValidator creates a MockMessagePoolValidator
func NewMockMessagePoolValidator() *MockMessagePoolValidator {
	return &MockMessagePoolValidator{Valid: true}
}

// Validate returns true if the mock validator is set to validate the message
func (v *MockMessagePoolValidator) Validate(ctx context.Context, msg *types.SignedMessage) error {
	if v.Valid {
		return nil
	}
	return errors.New("mock validation error")
}

// VMStorage creates a new storage object backed by an in memory datastore
func VMStorage() vm.StorageMap {
	return vm.NewStorageMap(blockstore.NewBlockstore(datastore.NewMapDatastore()))
}

// CreateTestMiner creates a new test miner with the given peerID and miner
// owner address within the state tree defined by st and vms with 100 FIL as
// collateral.
func CreateTestMiner(t *testing.T, st state.Tree, vms vm.StorageMap, minerOwnerAddr address.Address, pid peer.ID) address.Address {
	return CreateTestMinerWith(types.NewAttoFILFromFIL(100), t, st, vms, minerOwnerAddr, pid, 0)
}

// CreateTestMinerWith creates a new test miner with the given peerID miner
// owner address and collateral within the state tree defined by st and vms.
func CreateTestMinerWith(
	collateral types.AttoFIL,
	t *testing.T,
	stateTree state.Tree,
	vms vm.StorageMap,
	minerOwnerAddr address.Address,
	pid peer.ID,
	height uint64,
) address.Address {
	pdata := actor.MustConvertParams(types.OneKiBSectorSize, pid)
	nonce := RequireGetNonce(t, stateTree, address.TestAddress)
	msg := types.NewMessage(minerOwnerAddr, address.StorageMarketAddress, nonce, collateral, "createStorageMiner", pdata)

	result, err := ApplyTestMessage(stateTree, vms, msg, types.NewBlockHeight(height))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NoError(t, result.ExecutionError)
	addr, err := address.NewFromBytes(result.Receipt.Return[0])
	require.NoError(t, err)
	return addr
}

// GetTotalPower get total miner power from storage market
func GetTotalPower(t *testing.T, st state.Tree, vms vm.StorageMap) *types.BytesAmount {
	res, err := CreateAndApplyTestMessage(t, st, vms, address.StorageMarketAddress, 0, 0, "getTotalStorage", nil)
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)
	require.Equal(t, 1, len(res.Receipt.Return))
	return types.NewBytesAmountFromBytes(res.Receipt.Return[0])
}

// RequireGetNonce returns the next nonce of the actor at address a within
// state tree st, failing on error.
func RequireGetNonce(t *testing.T, st state.Tree, a address.Address) uint64 {
	ctx := context.Background()
	actr, err := st.GetActor(ctx, a)
	require.NoError(t, err)

	nonce, err := actor.NextNonce(actr)
	require.NoError(t, err)
	return nonce
}

// RequireCreateStorages creates an empty state tree and storage map.
func RequireCreateStorages(ctx context.Context, t *testing.T) (state.Tree, vm.StorageMap) {
	cst := hamt.NewCborStore()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)
	blk, err := consensus.DefaultGenesis(cst, bs)
	require.NoError(t, err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	require.NoError(t, err)

	vms := vm.NewStorageMap(bs)

	return st, vms
}

type testStorage struct {
	state interface{}
}

var _ exec.Storage = testStorage{}

// Put satisfies the Storage interface but does nothing.
func (ts testStorage) Put(v interface{}) (cid.Cid, error) {
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

// FakeVMContext creates the scaffold for faking out the vm context for direct calls to actors
type FakeVMContext struct {
	MessageValue            *types.Message
	StorageValue            exec.Storage
	BalanceValue            types.AttoFIL
	BlockHeightValue        *types.BlockHeight
	VerifierValue           verification.Verifier
	RandomnessValue         []byte
	IsFromAccountActorValue bool
	Sender                  func(to address.Address, method string, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error)
	Addresser               func() (address.Address, error)
	Charger                 func(cost types.GasUnits) error
	Sampler                 func(sampleHeight *types.BlockHeight) ([]byte, error)
	ActorCreator            func(addr address.Address, code cid.Cid, initalizationParams interface{}) error
}

var _ exec.VMContext = &FakeVMContext{}

// NewFakeVMContext fakes the state machine infrastructure so actor methods can be called directly
func NewFakeVMContext(message *types.Message, state interface{}) *FakeVMContext {
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
		Sender: func(to address.Address, method string, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
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
func NewFakeVMContextWithVerifier(message *types.Message, state interface{}, verifier verification.Verifier) *FakeVMContext {
	vmctx := NewFakeVMContext(message, state)
	vmctx.VerifierValue = verifier
	return vmctx
}

// Message is the message that triggered this invocation
func (tc *FakeVMContext) Message() *types.Message {
	return tc.MessageValue
}

// Storage provides and interface to actor state
func (tc *FakeVMContext) Storage() exec.Storage {
	return tc.StorageValue
}

// Send sends a message to another actor
func (tc *FakeVMContext) Send(to address.Address, method string, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
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
