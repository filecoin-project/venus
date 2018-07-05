package miner_test

import (
	"context"
	"math/big"
	"testing"

	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/actor"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/stretchr/testify/require"
)

func createTestMiner(assert *assert.Assertions, st state.Tree, vms *vm.StorageMap, minerOwnerAddr types.Address, key []byte, pid peer.ID) types.Address {
	pdata := actor.MustConvertParams(types.NewBytesAmount(10000), key, pid)
	nonce := core.MustGetNonce(st, address.TestAddress)
	msg := types.NewMessage(minerOwnerAddr, address.StorageMarketAddress, nonce, types.NewAttoFILFromFIL(100), "createMiner", pdata)

	result, err := core.ApplyMessage(context.Background(), st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err)

	addr, err := types.NewAddressFromBytes(result.Receipt.Return[0])
	assert.NoError(err)
	return addr
}

func TestAddAsk(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	minerAddr := createTestMiner(assert, st, vms, address.TestAddress, []byte{}, core.RequireRandomPeerID())

	// make an ask, and then make sure it all looks good
	pdata := actor.MustConvertParams(types.NewAttoFILFromFIL(100), types.NewBytesAmount(150))
	msg := types.NewMessage(address.TestAddress, minerAddr, 1, nil, "addAsk", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Equal(0, big.NewInt(0).Cmp(big.NewInt(0).SetBytes(result.Receipt.Return[0])))

	storageMkt, err := st.GetActor(ctx, address.StorageMarketAddress)
	assert.NoError(err)

	var strgMktStorage storagemarket.Storage
	assert.NoError(actor.UnmarshalStorage(storageMkt.ReadStorage(), &strgMktStorage))
	assert.Len(strgMktStorage.Orderbook.Asks, 1)
	assert.Equal(minerAddr, strgMktStorage.Orderbook.Asks[0].Owner)

	miner, err := st.GetActor(ctx, minerAddr)
	assert.NoError(err)

	var minerStorage Storage
	assert.NoError(actor.UnmarshalStorage(miner.ReadStorage(), &minerStorage))
	assert.Equal(types.NewBytesAmount(150), minerStorage.LockedStorage)

	// make another ask!
	pdata = actor.MustConvertParams(types.NewAttoFILFromFIL(110), types.NewBytesAmount(200))
	msg = types.NewMessage(address.TestAddress, minerAddr, 2, nil, "addAsk", pdata)

	result, err = core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Equal(big.NewInt(1), big.NewInt(0).SetBytes(result.Receipt.Return[0]))

	storageMkt, err = st.GetActor(ctx, address.StorageMarketAddress)
	assert.NoError(err)

	var strgMktStorage2 storagemarket.Storage
	assert.NoError(actor.UnmarshalStorage(storageMkt.ReadStorage(), &strgMktStorage2))
	assert.Len(strgMktStorage2.Orderbook.Asks, 2)
	assert.Equal(minerAddr, strgMktStorage2.Orderbook.Asks[1].Owner)

	miner, err = st.GetActor(ctx, minerAddr)
	assert.NoError(err)

	var minerStorage2 Storage
	assert.NoError(actor.UnmarshalStorage(miner.ReadStorage(), &minerStorage2))
	assert.Equal(types.NewBytesAmount(350), minerStorage2.LockedStorage)

	// now try to create an ask larger than our pledge
	pdata = actor.MustConvertParams(big.NewInt(55), types.NewBytesAmount(9900))
	msg = types.NewMessage(address.TestAddress, minerAddr, 3, nil, "addAsk", pdata)

	result, err = core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Contains(result.ExecutionError.Error(), Errors[ErrInsufficientPledge].Error())
}

func TestGetKey(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	signature := []byte("my public key")
	minerAddr := createTestMiner(assert, st, vms, address.TestAddress, signature, core.RequireRandomPeerID())

	// retrieve key
	result, exitCode, err := core.CallQueryMethod(ctx, st, minerAddr, "getKey", []byte{}, address.TestAddress, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Equal(uint8(0), exitCode)
	assert.Equal(result[0], signature)
}

func TestPeerIdGetterAndSetter(t *testing.T) {
	t.Run("successfully retrieves and updates peer ID", func(t *testing.T) {
		require := require.New(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := core.CreateStorages(ctx, t)

		origPid := core.RequireRandomPeerID()
		minerAddr := createTestMiner(assert.New(t), st, vms, address.TestAddress, []byte("my public key"), origPid)

		// retrieve peer ID
		retPidA := getPeerIdSuccess(ctx, t, st, address.TestAddress, minerAddr)
		require.Equal(peer.IDB58Encode(origPid), peer.IDB58Encode(retPidA))

		// update peer ID
		newPid := core.RequireRandomPeerID()
		updatePeerIdSuccess(ctx, t, st, vms, address.TestAddress, minerAddr, newPid)

		// retrieve peer ID
		retPidB := getPeerIdSuccess(ctx, t, st, address.TestAddress, minerAddr)
		require.Equal(peer.IDB58Encode(newPid), peer.IDB58Encode(retPidB))
	})

	t.Run("authorization failure while updating peer ID", func(t *testing.T) {
		require := require.New(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := core.CreateStorages(ctx, t)

		minerAddr := createTestMiner(assert.New(t), st, vms, address.TestAddress, []byte("other public key"), core.RequireRandomPeerID())

		// update peer ID and expect authorization failure (TestAddress2 doesn't owner miner)
		updatePeerIdMsg := types.NewMessage(
			address.TestAddress2,
			minerAddr,
			core.MustGetNonce(st, address.TestAddress2),
			types.NewAttoFILFromFIL(0),
			"updatePeerID",
			actor.MustConvertParams(core.RequireRandomPeerID()))

		applyMsgResult, err := core.ApplyMessage(ctx, st, vms, updatePeerIdMsg, types.NewBlockHeight(0))
		require.NoError(err)
		require.Equal(Errors[ErrCallerUnauthorized], applyMsgResult.ExecutionError)
		require.NotEqual(uint8(0), applyMsgResult.Receipt.ExitCode)
	})
}

func updatePeerIdSuccess(ctx context.Context, t *testing.T, st state.Tree, vms *vm.StorageMap, fromAddr types.Address, minerAddr types.Address, newPid peer.ID) {
	require := require.New(t)

	updatePeerIdMsg := types.NewMessage(
		fromAddr,
		minerAddr,
		core.MustGetNonce(st, fromAddr),
		types.NewAttoFILFromFIL(0),
		"updatePeerID",
		actor.MustConvertParams(newPid))

	applyMsgResult, err := core.ApplyMessage(ctx, st, vms, updatePeerIdMsg, types.NewBlockHeight(0))
	require.NoError(err)
	require.NoError(applyMsgResult.ExecutionError)
	require.Equal(uint8(0), applyMsgResult.Receipt.ExitCode)
}

func getPeerIdSuccess(ctx context.Context, t *testing.T, st state.Tree, fromAddr types.Address, minerAddr types.Address) peer.ID {
	res, code, err := core.CallQueryMethod(ctx, st, minerAddr, "getPeerID", []byte{}, fromAddr, nil)
	require.NoError(t, err)
	require.Equal(t, uint8(0), code)

	pid, err := peer.IDFromBytes(res[0])
	require.NoError(t, err)

	return pid
}
