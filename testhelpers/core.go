package testhelpers

import (
	"context"
	"errors"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
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

// MustSign signs a given address with the provided mocksigner or panics if it
// cannot.
func MustSign(s types.MockSigner, msgs ...*types.Message) []*types.SignedMessage {
	var smsgs []*types.SignedMessage
	for _, m := range msgs {
		gasLimit := types.NewGasUnits(999)
		sm, err := types.NewSignedMessage(*m, &s, types.NewGasPrice(0), gasLimit)
		if err != nil {
			panic(err)
		}
		smsgs = append(smsgs, sm)
	}
	return smsgs
}

// CreateTestMiner creates a new test miner with the given peerID and miner
// owner address within the state tree defined by st and vms with 100 FIL as
// collateral.
func CreateTestMiner(t *testing.T, st state.Tree, vms vm.StorageMap, minerOwnerAddr address.Address, pid peer.ID) address.Address {
	return CreateTestMinerWith(types.NewAttoFILFromFIL(100), t, st, vms, minerOwnerAddr, pid)
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
) address.Address {
	pdata := actor.MustConvertParams(types.OneKiBSectorSize, pid)
	nonce := RequireGetNonce(t, stateTree, address.TestAddress)
	msg := types.NewMessage(minerOwnerAddr, address.StorageMarketAddress, nonce, collateral, "createStorageMiner", pdata)

	result, err := ApplyTestMessage(stateTree, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)

	addr, err := address.NewFromBytes(result.Receipt.Return[0])
	require.NoError(t, err)
	return addr
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
