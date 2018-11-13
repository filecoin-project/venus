package miner_test

import (
	"context"
	"math/big"
	"testing"

	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/actor"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/stretchr/testify/require"
)

func createTestMiner(assert *assert.Assertions, st state.Tree, vms vm.StorageMap, minerOwnerAddr address.Address, key []byte, pid peer.ID) address.Address {
	pdata := actor.MustConvertParams(big.NewInt(100), key, pid)
	nonce := core.MustGetNonce(st, address.TestAddress)
	msg := types.NewMessage(minerOwnerAddr, address.StorageMarketAddress, nonce, types.NewAttoFILFromFIL(100), "createMiner", pdata)

	result, err := consensus.ApplyMessage(context.Background(), st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err)

	addr, err := address.NewFromBytes(result.Receipt.Return[0])
	assert.NoError(err)
	return addr
}

func TestAddAsk(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	minerAddr := createTestMiner(assert, st, vms, address.TestAddress, []byte{}, th.RequireRandomPeerID())

	// make an ask, and then make sure it all looks good
	pdata := actor.MustConvertParams(types.NewAttoFILFromFIL(5), big.NewInt(1500))
	msg := types.NewMessage(address.TestAddress, minerAddr, 1, nil, "addAsk", pdata)

	_, err := consensus.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(1))
	assert.NoError(err)

	pdata = actor.MustConvertParams(big.NewInt(0))
	msg = types.NewMessage(address.TestAddress, minerAddr, 2, types.NewZeroAttoFIL(), "getAsk", pdata)
	result, err := consensus.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(2))
	assert.NoError(err)

	var ask Ask
	err = actor.UnmarshalStorage(result.Receipt.Return[0], &ask)
	require.NoError(err)
	assert.Equal(types.NewBlockHeight(1501), ask.Expiry)

	miner, err := st.GetActor(ctx, minerAddr)
	assert.NoError(err)

	var minerStorage State
	builtin.RequireReadState(t, vms, minerAddr, miner, &minerStorage)
	assert.Equal(1, len(minerStorage.Asks))
	assert.Equal(uint64(1), minerStorage.NextAskID.Uint64())

	// make another ask!
	pdata = actor.MustConvertParams(types.NewAttoFILFromFIL(110), big.NewInt(200))
	msg = types.NewMessage(address.TestAddress, minerAddr, 3, nil, "addAsk", pdata)
	result, err = consensus.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(3))
	assert.NoError(err)
	assert.Equal(big.NewInt(1), big.NewInt(0).SetBytes(result.Receipt.Return[0]))

	pdata = actor.MustConvertParams(big.NewInt(1))
	msg = types.NewMessage(address.TestAddress, minerAddr, 4, types.NewZeroAttoFIL(), "getAsk", pdata)
	result, err = consensus.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(4))
	assert.NoError(err)

	var ask2 Ask
	err = actor.UnmarshalStorage(result.Receipt.Return[0], &ask2)
	require.NoError(err)
	assert.Equal(types.NewBlockHeight(203), ask2.Expiry)
	assert.Equal(uint64(1), ask2.ID.Uint64())

	msg = types.NewMessage(address.TestAddress, minerAddr, 5, types.NewZeroAttoFIL(), "getAsks", nil)
	result, err = consensus.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(4))
	assert.NoError(err)
	assert.NoError(result.ExecutionError)

	var askids []uint64
	require.NoError(actor.UnmarshalStorage(result.Receipt.Return[0], &askids))
	assert.Len(askids, 2)
}

func TestGetKey(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	signature := []byte("my public key")
	minerAddr := createTestMiner(assert, st, vms, address.TestAddress, signature, th.RequireRandomPeerID())

	// retrieve key
	result, exitCode, err := consensus.CallQueryMethod(ctx, st, vms, minerAddr, "getKey", []byte{}, address.TestAddress, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Equal(uint8(0), exitCode)
	assert.Equal(result[0], signature)
}

func TestCBOREncodeState(t *testing.T) {
	assert := assert.New(t)
	state := NewState(address.TestAddress, []byte{}, big.NewInt(1), th.RequireRandomPeerID(), types.NewZeroAttoFIL())

	state.Sectors["1"] = &Commitments{
		CommR: [32]byte{},
		CommD: [32]byte{},
	}

	_, err := actor.MarshalStorage(state)
	assert.NoError(err)

}

func TestPeerIdGetterAndSetter(t *testing.T) {
	t.Run("successfully retrieves and updates peer ID", func(t *testing.T) {
		require := require.New(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := core.CreateStorages(ctx, t)

		origPid := th.RequireRandomPeerID()
		minerAddr := createTestMiner(assert.New(t), st, vms, address.TestAddress, []byte("my public key"), origPid)

		// retrieve peer ID
		retPidA := getPeerIdSuccess(ctx, t, st, vms, address.TestAddress, minerAddr)
		require.Equal(peer.IDB58Encode(origPid), peer.IDB58Encode(retPidA))

		// update peer ID
		newPid := th.RequireRandomPeerID()
		updatePeerIdSuccess(ctx, t, st, vms, address.TestAddress, minerAddr, newPid)

		// retrieve peer ID
		retPidB := getPeerIdSuccess(ctx, t, st, vms, address.TestAddress, minerAddr)
		require.Equal(peer.IDB58Encode(newPid), peer.IDB58Encode(retPidB))
	})

	t.Run("authorization failure while updating peer ID", func(t *testing.T) {
		require := require.New(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := core.CreateStorages(ctx, t)

		minerAddr := createTestMiner(assert.New(t), st, vms, address.TestAddress, []byte("other public key"), th.RequireRandomPeerID())

		// update peer ID and expect authorization failure (TestAddress2 doesn't owner miner)
		updatePeerIdMsg := types.NewMessage(
			address.TestAddress2,
			minerAddr,
			core.MustGetNonce(st, address.TestAddress2),
			types.NewAttoFILFromFIL(0),
			"updatePeerID",
			actor.MustConvertParams(th.RequireRandomPeerID()))

		applyMsgResult, err := consensus.ApplyMessage(ctx, st, vms, updatePeerIdMsg, types.NewBlockHeight(0))
		require.NoError(err)
		require.Equal(Errors[ErrCallerUnauthorized], applyMsgResult.ExecutionError)
		require.NotEqual(uint8(0), applyMsgResult.Receipt.ExitCode)
	})
}

func updatePeerIdSuccess(ctx context.Context, t *testing.T, st state.Tree, vms vm.StorageMap, fromAddr address.Address, minerAddr address.Address, newPid peer.ID) {
	require := require.New(t)

	updatePeerIdMsg := types.NewMessage(
		fromAddr,
		minerAddr,
		core.MustGetNonce(st, fromAddr),
		types.NewAttoFILFromFIL(0),
		"updatePeerID",
		actor.MustConvertParams(newPid))

	applyMsgResult, err := consensus.ApplyMessage(ctx, st, vms, updatePeerIdMsg, types.NewBlockHeight(0))
	require.NoError(err)
	require.NoError(applyMsgResult.ExecutionError)
	require.Equal(uint8(0), applyMsgResult.Receipt.ExitCode)
}

func getPeerIdSuccess(ctx context.Context, t *testing.T, st state.Tree, vms vm.StorageMap, fromAddr address.Address, minerAddr address.Address) peer.ID {
	res, code, err := consensus.CallQueryMethod(ctx, st, vms, minerAddr, "getPeerID", []byte{}, fromAddr, nil)
	require.NoError(t, err)
	require.Equal(t, uint8(0), code)

	pid, err := peer.IDFromBytes(res[0])
	require.NoError(t, err)

	return pid
}

func TestMinerCommitSector(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	st, vms := core.CreateStorages(ctx, t)

	origPid := th.RequireRandomPeerID()
	minerAddr := createTestMiner(assert.New(t), st, vms, address.TestAddress, []byte("my public key"), origPid)

	commR := th.MakeCommitment()
	commD := th.MakeCommitment()

	res, err := applyMessage(t, st, vms, minerAddr, 0, 3, "commitSector", uint64(1), commR, commD)
	require.NoError(err)
	require.NoError(res.ExecutionError)
	require.Equal(uint8(0), res.Receipt.ExitCode)

	// check that the proving period matches
	res, err = applyMessage(t, st, vms, minerAddr, 0, 3, "getProvingPeriodStart")
	require.NoError(err)
	require.NoError(res.ExecutionError)
	// blockheight was 3
	require.Equal(types.NewBlockHeight(3), types.NewBlockHeightFromBytes(res.Receipt.Return[0]))

	// fail because commR already exists
	res, err = applyMessage(t, st, vms, minerAddr, 0, 4, "commitSector", uint64(1), commR, commD)
	require.NoError(err)
	require.EqualError(res.ExecutionError, "sector already committed")
	require.Equal(uint8(0x23), res.Receipt.ExitCode)
}

func TestMinerSubmitPoSt(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	st, vms := core.CreateStorages(ctx, t)

	origPid := th.RequireRandomPeerID()
	minerAddr := createTestMiner(assert.New(t), st, vms, address.TestAddress, []byte("my public key"), origPid)

	// add a sector
	res, err := applyMessage(t, st, vms, minerAddr, 0, 3, "commitSector", uint64(1), th.MakeCommitment(), th.MakeCommitment())
	require.NoError(err)
	require.NoError(res.ExecutionError)
	require.Equal(uint8(0), res.Receipt.ExitCode)

	// add another sector
	res, err = applyMessage(t, st, vms, minerAddr, 0, 4, "commitSector", uint64(2), th.MakeCommitment(), th.MakeCommitment())
	require.NoError(err)
	require.NoError(res.ExecutionError)
	require.Equal(uint8(0), res.Receipt.ExitCode)

	// submit post
	res, err = applyMessage(t, st, vms, minerAddr, 0, 8, "submitPoSt", th.MakeProof())
	require.NoError(err)
	require.NoError(res.ExecutionError)
	require.Equal(uint8(0), res.Receipt.ExitCode)

	// check that the proving period is now the next one
	res, err = applyMessage(t, st, vms, minerAddr, 0, 9, "getProvingPeriodStart")
	require.NoError(err)
	require.NoError(res.ExecutionError)
	require.Equal(types.NewBlockHeightFromBytes(res.Receipt.Return[0]), types.NewBlockHeight(20003))

	// fail to submit inside the proving period
	res, err = applyMessage(t, st, vms, minerAddr, 0, 40008, "submitPoSt", th.MakeProof())
	require.NoError(err)
	require.EqualError(res.ExecutionError, "submitted PoSt late, need to pay a fee")
}

func applyMessage(t *testing.T, st state.Tree, vms vm.StorageMap, to address.Address, val, bh uint64, method string, params ...interface{}) (*consensus.ApplicationResult, error) {
	t.Helper()

	pdata := actor.MustConvertParams(params...)
	nonce := core.MustGetNonce(st, address.TestAddress)
	msg := types.NewMessage(address.TestAddress, to, nonce, types.NewAttoFILFromFIL(val), method, pdata)
	return consensus.ApplyMessage(context.Background(), st, vms, msg, types.NewBlockHeight(bh))
}
