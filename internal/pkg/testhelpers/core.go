package testhelpers

import (
	"context"
	"errors"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/account"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	"github.com/stretchr/testify/require"
)

// RequireMakeStateTree takes a map of addresses to actors and stores them on
// the state tree, requiring that all its steps succeed.
func RequireMakeStateTree(t *testing.T, cst *hamt.CborIpldStore, acts map[address.Address]*actor.Actor) (cid.Cid, state.Tree) {
	ctx := context.Background()
	tree := state.NewTree(cst)

	for addr, act := range acts {
		err := tree.SetActor(ctx, addr, act)
		require.NoError(t, err)
	}

	c, err := tree.Flush(ctx)
	require.NoError(t, err)

	return c, tree
}

// RequireNewMinerActor creates a new miner actor with the given owner, pledge, and collateral,
// and requires that its steps succeed.
func RequireNewMinerActor(ctx context.Context, t *testing.T, st state.Tree, vms vm.Storage, owner address.Address, pledge uint64, pid peer.ID, coll types.AttoFIL) (*actor.Actor, address.Address) {
	// Dragons: re-write

	// cachedTree := state.NewCachedTree(st)
	// initActor, _, err := cachedTree.GetOrCreateActor(ctx, address.InitAddress, func() (*actor.Actor, address.Address, error) {
	// 	return RequireNewInitActor(t, vms), address.InitAddress, nil
	// })
	// require.NoError(t, err)

	// gt := vm.NewLegacyGasTracker()
	// gt.MsgGasLimit = types.NewGasUnits(10000)
	// // we are required to have a message in the context even though it will not be used.
	// dummyMessage := types.NewUnsignedMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, types.SendMethodID, []byte{})
	// vmctx := vm.NewVMContext(vm.NewContextParams{
	// 	Message:    dummyMessage,
	// 	OriginMsg:  dummyMessage,
	// 	State:      cachedTree,
	// 	StorageMap: vms,
	// 	To:         initActor,
	// 	ToAddr:     address.InitAddress,
	// 	GasTracker: gt,
	// 	Actors:     builtin.DefaultActors,
	// })

	// actorAddr, _, err := (&initactor.Impl{}).Exec(vmctx, types.MinerActorCodeCid, []interface{}{owner, owner, pid, types.OneKiBSectorSize})
	// require.NoError(t, err)

	// require.NoError(t, cachedTree.Commit(ctx))
	// require.NoError(t, vms.Flush())

	// return RequireLookupActor(ctx, t, st, vms, actorAddr)

	return nil, address.Undef
}

// RequireLookupActor converts the given address to an id address before looking up the actor in the state tree
func RequireLookupActor(ctx context.Context, t *testing.T, st state.Tree, vms vm.Storage, actorAddr address.Address) (*actor.Actor, address.Address) {
	// Dragons: delete

	// if actorAddr.Protocol() == address.ID {
	// 	return state.MustGetActor(st, actorAddr), actorAddr
	// }

	// cachedTree := state.NewCachedTree(st)
	// initActor, _, err := cachedTree.GetOrCreateActor(ctx, address.InitAddress, func() (*actor.Actor, address.Address, error) {
	// 	return RequireNewInitActor(t, vms), address.InitAddress, nil
	// })
	// require.NoError(t, err)

	// vmctx := vm.NewVMContext(vm.NewContextParams{
	// 	State:      cachedTree,
	// 	StorageMap: vms,
	// 	To:         initActor,
	// 	ToAddr:     address.InitAddress,
	// })
	// id, found, err := initactor.LookupIDAddress(vmctx, actorAddr)
	// require.NoError(t, err)
	// require.True(t, found)

	// idAddr, err := address.NewIDAddress(id)
	// require.NoError(t, err)

	// act, err := cachedTree.GetActor(ctx, idAddr)
	// require.NoError(t, err)

	// return act, idAddr

	return nil, address.Undef
}

// RequireNewAccountActor creates a new account actor without adding it to state
func RequireNewAccountActor(t *testing.T, value types.AttoFIL) *actor.Actor {
	act, err := account.NewActor(value)
	require.NoError(t, err)
	return act
}

// RequireNewFakeActor instantiates and returns a new fake actor and requires
// that its steps succeed.
func RequireNewFakeActor(t *testing.T, vms vm.Storage, addr address.Address, codeCid cid.Cid) *actor.Actor {
	return RequireNewFakeActorWithTokens(t, vms, addr, codeCid, types.NewAttoFILFromFIL(100))
}

// RequireNewFakeActorWithTokens instantiates and returns a new fake actor and requires
// that its steps succeed.
func RequireNewFakeActorWithTokens(t *testing.T, vms vm.Storage, addr address.Address, codeCid cid.Cid, amt types.AttoFIL) *actor.Actor {
	act := actor.NewActor(codeCid, amt)
	var err error
	act.Head, err = (&actor.FakeActor{}).InitializeState(vms, &actor.FakeActorStorage{})
	require.NoError(t, err)
	require.NoError(t, vms.Flush())
	return act
}

// RequireNewInitActor instantiates and returns a new init actor
func RequireNewInitActor(t *testing.T, vms vm.Storage) *actor.Actor {
	// Dragons: why do we need this? remove
	// act := actor.NewActor(types.InitActorCodeCid, types.ZeroAttoFIL)
	// store := vms.NewStorage(address.InitAddress, act)
	// err := (&initactor.Actor{}).InitializeState(store, "test")
	// require.NoError(t, err)
	// require.NoError(t, vms.Flush())
	// return act
	return nil
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
func VMStorage() vm.Storage {
	return vm.NewStorage(blockstore.NewBlockstore(datastore.NewMapDatastore()))
}

// CreateTestMiner creates a new test miner with the given peerID and miner
// owner address within the state tree defined by st and vms with 100 FIL as
// collateral.
func CreateTestMiner(t *testing.T, st state.Tree, vms vm.Storage, minerOwnerAddr address.Address, pid peer.ID) address.Address {
	return CreateTestMinerWith(types.NewAttoFILFromFIL(100), t, st, vms, minerOwnerAddr, pid, 0)
}

// CreateTestMinerWith creates a new test miner with the given peerID miner
// owner address and collateral within the state tree defined by st and vms.
func CreateTestMinerWith(
	collateral types.AttoFIL,
	t *testing.T,
	st state.Tree,
	vms vm.Storage,
	minerOwnerAddr address.Address,
	pid peer.ID,
	height uint64,
) address.Address {
	ctx := context.TODO()
	pdata := actor.MustConvertParams(types.OneKiBSectorSize, pid)
	idAddr, found, err := consensus.ResolveAddress(ctx, minerOwnerAddr, state.NewCachedTree(st), vms)
	require.NoError(t, err)
	require.True(t, found)

	nonce := RequireGetNonce(t, st, vms, idAddr)
	msg := types.NewUnsignedMessage(minerOwnerAddr, address.StorageMarketAddress, nonce, collateral, storagemarket.CreateStorageMiner, pdata)

	result, err := ApplyTestMessage(st, vms, msg, types.NewBlockHeight(height))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NoError(t, result.ExecutionError)
	addr, err := address.NewFromBytes(result.Receipt.Return[0])
	require.NoError(t, err)
	return addr
}

// RequireGetActor adds an account actor to the state tree if none exists
func RequireGetActor(ctx context.Context, t *testing.T, st state.Tree, vms vm.Storage, addr address.Address) *actor.Actor {
	idAddr, found, err := consensus.ResolveAddress(ctx, addr, state.NewCachedTree(st), vms)
	require.NoError(t, err)
	require.True(t, found, "test actor not found")

	act, err := st.GetActor(ctx, idAddr)
	require.NoError(t, err)
	return act
}

// RequireInitAccountActor initializes an account actor
func RequireInitAccountActor(ctx context.Context, t *testing.T, st state.Tree, vms vm.Storage, addr address.Address, balance types.AttoFIL) (*actor.Actor, address.Address) {
	// Dragons: how do you NOT have an init actor on the state? Delete this method

	// cachedTree := state.NewCachedTree(st)

	// // ensure network actor
	// network, _, err := cachedTree.GetOrCreateActor(ctx, address.LegacyNetworkAddress, func() (*actor.Actor, address.Address, error) {
	// 	act, err := account.NewActor(types.NewAttoFILFromFIL(100000000))
	// 	return act, address.LegacyNetworkAddress, err
	// })
	// require.NoError(t, err)

	// // ensure init actor
	// _, _, err = cachedTree.GetOrCreateActor(ctx, address.InitAddress, func() (*actor.Actor, address.Address, error) {
	// 	return RequireNewInitActor(t, vms), address.InitAddress, nil
	// })
	// require.NoError(t, err)

	// // create actor
	// vmctx := vm.NewVMContext(vm.NewContextParams{Actors: builtin.DefaultActors, State: cachedTree, StorageMap: vms, To: network, ToAddr: address.LegacyNetworkAddress})
	// vmctx.Send(address.InitAddress, initactor.ExecMethodID, balance, []interface{}{types.AccountActorCodeCid, []interface{}{addr}})

	// // fetch id address for actor from init actor
	// gt := vm.NewLegacyGasTracker()
	// gt.MsgGasLimit = 10000
	// vmctx = vm.NewVMContext(vm.NewContextParams{Actors: builtin.DefaultActors, State: cachedTree, StorageMap: vms, To: network, ToAddr: address.LegacyNetworkAddress, GasTracker: gt})
	// idAddrInt := vmctx.Send(address.InitAddress, initactor.GetActorIDForAddressMethodID, types.ZeroAttoFIL, []interface{}{addr})

	// idAddr, err := address.NewIDAddress(idAddrInt.(*big.Int).Uint64())
	// require.NoError(t, err)

	// act, err := cachedTree.GetActor(ctx, idAddr)
	// require.NoError(t, err)

	// require.NoError(t, cachedTree.Commit(ctx))
	// require.NoError(t, vms.Flush())

	// return act, idAddr

	return nil, address.Undef
}

// GetTotalPower get total miner power from storage market
func GetTotalPower(t *testing.T, st state.Tree, vms vm.Storage) *types.BytesAmount {
	res, err := CreateAndApplyTestMessage(t, st, vms, address.StorageMarketAddress, 0, 0, storagemarket.GetTotalStorage, nil)
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)
	require.Equal(t, 1, len(res.Receipt.Return))
	return types.NewBytesAmountFromBytes(res.Receipt.Return[0])
}

// RequireGetNonce returns the next nonce of the actor at address a within
// state tree st, failing on error.
func RequireGetNonce(t *testing.T, st state.Tree, vms vm.Storage, a address.Address) uint64 {
	ctx := context.Background()
	actr, _ := RequireLookupActor(ctx, t, st, vms, a)
	nonce, err := actor.NextNonce(actr)
	require.NoError(t, err)
	return nonce
}

// RequireCreateStorages creates an empty state tree and storage map.
func RequireCreateStorages(ctx context.Context, t *testing.T) (state.Tree, vm.Storage) {
	cst := hamt.NewCborStore()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)
	blk, err := DefaultGenesis(cst, bs)
	require.NoError(t, err)

	st, err := state.NewTreeLoader().LoadStateTree(ctx, cst, blk.StateRoot)
	require.NoError(t, err)

	vms := vm.NewStorage(bs)

	return st, vms
}

// DefaultGenesis creates a test network genesis block with default accounts and actors installed.
func DefaultGenesis(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*block.Block, error) {
	return consensus.MakeGenesisFunc(consensus.Network(version.TEST))(cst, bs)
}
