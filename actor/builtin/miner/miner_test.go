package miner_test

import (
	"context"
	"math/big"
	"testing"

	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

func createTestMiner(t *testing.T, st state.Tree, vms vm.StorageMap, minerOwnerAddr address.Address, key []byte, pid peer.ID) address.Address {
	return createTestMinerWith(100, 100, t, st, vms, minerOwnerAddr, key, pid)
}

func createTestMinerWith(pledge int64,
	collateral uint64,
	t *testing.T,
	stateTree state.Tree,
	vms vm.StorageMap,
	minerOwnerAddr address.Address,
	key []byte,
	peerId peer.ID,
) address.Address {
	pdata := actor.MustConvertParams(big.NewInt(pledge), key, peerId)
	nonce := core.MustGetNonce(stateTree, address.TestAddress)
	msg := types.NewMessage(minerOwnerAddr, address.StorageMarketAddress, nonce, types.NewAttoFILFromFIL(collateral), "createMiner", pdata)

	result, err := th.ApplyTestMessage(stateTree, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)

	addr, err := address.NewFromBytes(result.Receipt.Return[0])
	assert.NoError(t, err)
	return addr
}

func TestAskFunctions(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	minerAddr := createTestMiner(t, st, vms, address.TestAddress, []byte("abcd123"), th.RequireRandomPeerID(t))

	// make an ask, and then make sure it all looks good
	pdata := actor.MustConvertParams(types.NewAttoFILFromFIL(5), big.NewInt(1500))
	msg := types.NewMessage(address.TestAddress, minerAddr, 1, nil, "addAsk", pdata)

	_, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(1))
	assert.NoError(t, err)

	pdata = actor.MustConvertParams(big.NewInt(0))
	msg = types.NewMessage(address.TestAddress, minerAddr, 2, types.NewZeroAttoFIL(), "getAsk", pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(2))
	assert.NoError(t, err)

	var ask Ask
	err = actor.UnmarshalStorage(result.Receipt.Return[0], &ask)
	require.NoError(t, err)
	assert.Equal(t, types.NewBlockHeight(1501), ask.Expiry)

	miner, err := st.GetActor(ctx, minerAddr)
	assert.NoError(t, err)

	var minerStorage State
	builtin.RequireReadState(t, vms, minerAddr, miner, &minerStorage)
	assert.Equal(t, 1, len(minerStorage.Asks))
	assert.Equal(t, uint64(1), minerStorage.NextAskID.Uint64())

	// Look for an ask that doesn't exist
	pdata = actor.MustConvertParams(big.NewInt(3453))
	msg = types.NewMessage(address.TestAddress, minerAddr, 2, types.NewZeroAttoFIL(), "getAsk", pdata)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(2))
	assert.Equal(t, Errors[ErrAskNotFound], result.ExecutionError)
	assert.NoError(t, err)

	// make another ask!
	pdata = actor.MustConvertParams(types.NewAttoFILFromFIL(110), big.NewInt(200))
	msg = types.NewMessage(address.TestAddress, minerAddr, 3, nil, "addAsk", pdata)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(3))
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(1), big.NewInt(0).SetBytes(result.Receipt.Return[0]))

	pdata = actor.MustConvertParams(big.NewInt(1))
	msg = types.NewMessage(address.TestAddress, minerAddr, 4, types.NewZeroAttoFIL(), "getAsk", pdata)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(4))
	assert.NoError(t, err)

	var ask2 Ask
	err = actor.UnmarshalStorage(result.Receipt.Return[0], &ask2)
	require.NoError(t, err)
	assert.Equal(t, types.NewBlockHeight(203), ask2.Expiry)
	assert.Equal(t, uint64(1), ask2.ID.Uint64())

	msg = types.NewMessage(address.TestAddress, minerAddr, 5, types.NewZeroAttoFIL(), "getAsks", nil)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(4))
	assert.NoError(t, err)
	assert.NoError(t, result.ExecutionError)

	var askids []uint64
	require.NoError(t, actor.UnmarshalStorage(result.Receipt.Return[0], &askids))
	assert.Len(t, askids, 2)
}

func TestGetKey(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	signature := []byte("my public key")
	minerAddr := createTestMiner(t, st, vms, address.TestAddress, signature, th.RequireRandomPeerID(t))

	// retrieve key
	result := callQueryMethodSuccess("getKey", ctx, t, st, vms, address.TestAddress, minerAddr)
	assert.Equal(t, result[0], signature)
}

func TestCBOREncodeState(t *testing.T) {
	tf.UnitTest(t)

	state := NewState(address.TestAddress, []byte{}, big.NewInt(1), th.RequireRandomPeerID(t), types.NewZeroAttoFIL(), types.OneKiBSectorSize)

	state.SectorCommitments["1"] = types.Commitments{
		CommD:     types.CommD{},
		CommR:     types.CommR{},
		CommRStar: types.CommRStar{},
	}

	_, err := actor.MarshalStorage(state)
	assert.NoError(t, err)

}

func TestPeerIdGetterAndSetter(t *testing.T) {
	tf.UnitTest(t)

	t.Run("successfully retrieves and updates peer ID", func(t *testing.T) {

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := core.CreateStorages(ctx, t)

		origPid := th.RequireRandomPeerID(t)
		minerAddr := createTestMiner(t, st, vms, address.TestAddress, []byte("my public key"), origPid)

		// retrieve peer ID
		resultA := callQueryMethodSuccess("getPeerID", ctx, t, st, vms, address.TestAddress, minerAddr)
		pid, err := peer.IDFromBytes(resultA[0])
		require.NoError(t, err)

		require.Equal(t, peer.IDB58Encode(origPid), peer.IDB58Encode(pid))

		// update peer ID
		newPid := th.RequireRandomPeerID(t)
		updatePeerIdSuccess(ctx, t, st, vms, address.TestAddress, minerAddr, newPid)

		// retrieve peer ID
		resultB := callQueryMethodSuccess("getPeerID", ctx, t, st, vms, address.TestAddress, minerAddr)
		pid, err = peer.IDFromBytes(resultB[0])
		require.NoError(t, err)

		require.Equal(t, peer.IDB58Encode(newPid), peer.IDB58Encode(pid))
	})

	t.Run("authorization failure while updating peer ID", func(t *testing.T) {

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := core.CreateStorages(ctx, t)

		minerAddr := createTestMiner(t, st, vms, address.TestAddress, []byte("other public key"), th.RequireRandomPeerID(t))

		// update peer ID and expect authorization failure (TestAddress2 doesn't owner miner)
		updatePeerIdMsg := types.NewMessage(
			address.TestAddress2,
			minerAddr,
			core.MustGetNonce(st, address.TestAddress2),
			types.NewAttoFILFromFIL(0),
			"updatePeerID",
			actor.MustConvertParams(th.RequireRandomPeerID(t)))

		applyMsgResult, err := th.ApplyTestMessage(st, vms, updatePeerIdMsg, types.NewBlockHeight(0))
		require.NoError(t, err)
		require.Equal(t, Errors[ErrCallerUnauthorized], applyMsgResult.ExecutionError)
		require.NotEqual(t, uint8(0), applyMsgResult.Receipt.ExitCode)
	})
}

func TestMinerGetPledge(t *testing.T) {
	tf.UnitTest(t)

	t.Run("GetPledge returns pledged sectors, 0, nil when successful", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := core.CreateStorages(ctx, t)

		minerAddr := createTestMinerWith(120, 240, t, st, vms, address.TestAddress,
			[]byte("my public key"), th.RequireRandomPeerID(t))

		// retrieve power (trivial result for no proven sectors)
		result := callQueryMethodSuccess("getPledge", ctx, t, st, vms, address.TestAddress, minerAddr)[0][0]

		require.Equal(t, 120, int(result))
	})
}

func TestMinerGetPower(t *testing.T) {
	tf.UnitTest(t)

	t.Run("GetPower returns proven sectors, 0, nil when successful", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := core.CreateStorages(ctx, t)

		minerAddr := createTestMinerWith(120, 240, t, st, vms, address.TestAddress,
			[]byte("my public key"), th.RequireRandomPeerID(t))

		// retrieve power (trivial result for no proven sectors)
		result := callQueryMethodSuccess("getPower", ctx, t, st, vms, address.TestAddress, minerAddr)
		require.Equal(t, []byte{}, result[0])
	})
}

func updatePeerIdSuccess(ctx context.Context, t *testing.T, st state.Tree, vms vm.StorageMap, fromAddr address.Address, minerAddr address.Address, newPid peer.ID) {
	updatePeerIdMsg := types.NewMessage(
		fromAddr,
		minerAddr,
		core.MustGetNonce(st, fromAddr),
		types.NewAttoFILFromFIL(0),
		"updatePeerID",
		actor.MustConvertParams(newPid))

	applyMsgResult, err := th.ApplyTestMessage(st, vms, updatePeerIdMsg, types.NewBlockHeight(0))
	require.NoError(t, err)
	require.NoError(t, applyMsgResult.ExecutionError)
	require.Equal(t, uint8(0), applyMsgResult.Receipt.ExitCode)
}

func callQueryMethodSuccess(method string,
	ctx context.Context,
	t *testing.T, st state.Tree,
	vms vm.StorageMap,
	fromAddr address.Address,
	minerAddr address.Address) [][]byte {
	res, code, err := consensus.CallQueryMethod(ctx, st, vms, minerAddr, method, []byte{}, fromAddr, nil)
	require.NoError(t, err)
	require.Equal(t, uint8(0), code)
	return res
}

func TestMinerCommitSector(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	st, vms := core.CreateStorages(ctx, t)

	origPid := th.RequireRandomPeerID(t)
	minerAddr := createTestMiner(t, st, vms, address.TestAddress, []byte("my public key"), origPid)

	commR := th.MakeCommitment()
	commRStar := th.MakeCommitment()
	commD := th.MakeCommitment()

	res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, "commitSector", nil, uint64(1), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)

	// check that the proving period matches
	res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, "getProvingPeriodStart", nil)
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	// blockheight was 3
	require.Equal(t, types.NewBlockHeight(3), types.NewBlockHeightFromBytes(res.Receipt.Return[0]))

	// fail because commR already exists
	res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 4, "commitSector", nil, uint64(1), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.EqualError(t, res.ExecutionError, "sector already committed")
	require.Equal(t, uint8(0x23), res.Receipt.ExitCode)
}

func TestMinerSubmitPoSt(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	st, vms := core.CreateStorages(ctx, t)

	ancestors := th.RequireTipSetChain(t, 10)

	origPid := th.RequireRandomPeerID(t)
	minerAddr := createTestMiner(t, st, vms, address.TestAddress, []byte("my public key"), origPid)

	// add a sector
	res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, "commitSector", ancestors, uint64(1), th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)

	// add another sector
	res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 4, "commitSector", ancestors, uint64(2), th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)

	// submit post
	proof := th.MakeRandomPoSTProofForTest()
	res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 8, "submitPoSt", ancestors, []types.PoStProof{proof})
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)

	// check that the proving period is now the next one
	res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 9, "getProvingPeriodStart", ancestors)
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, types.NewBlockHeightFromBytes(res.Receipt.Return[0]), types.NewBlockHeight(20003))

	// fail to submit inside the proving period
	proof = th.MakeRandomPoSTProofForTest()
	res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 40008, "submitPoSt", ancestors, []types.PoStProof{proof})
	require.NoError(t, err)
	require.EqualError(t, res.ExecutionError, "submitted PoSt late, need to pay a fee")
}

func TestVerifyPIP(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	st, vms := core.CreateStorages(ctx, t)

	ancestors := th.RequireTipSetChain(t, 10)

	origPid := th.RequireRandomPeerID(t)
	minerAddr := createTestMiner(t, st, vms, address.TestAddress, []byte("my public key"), origPid)

	sectorId := uint64(1)
	commD := th.MakeCommitment()

	// add a sector
	res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, "commitSector", ancestors, sectorId, commD, th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)

	runVerifyPIP := func(t *testing.T, bh uint64, commP []byte, sectorId uint64, proof []byte) error {
		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, bh, "verifyPieceInclusion", ancestors, commP, sectorId, proof)
		require.NoError(t, err)

		return res.ExecutionError
	}

	commP := th.MakeCommitment()

	// TODO: This is a fake pip form by concatenating commP and commD.
	// It will need to be generated correctly once real verification is implemented
	// see https://github.com/filecoin-project/go-filecoin/issues/2629
	pip := []byte{}
	pip = append(pip, commP[:]...)
	pip = append(pip, commD[:]...)

	t.Run("PIP is invalid if miner has not submitted any proofs", func(t *testing.T) {
		err := runVerifyPIP(t, 3, commP, sectorId, pip)

		assert.Error(t, err)
		assert.Equal(t, "proofs out of date", err.Error())
	})

	t.Run("After submitting a PoSt", func(t *testing.T) {
		// submit a post
		proof := th.MakeRandomPoSTProofForTest()
		blockheightOfPoSt := uint64(8)
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, blockheightOfPoSt, "submitPoSt", ancestors, []types.PoStProof{proof})
		assert.NoError(t, err)
		assert.NoError(t, res.ExecutionError)
		assert.Equal(t, uint8(0), res.Receipt.ExitCode)

		t.Run("Valid PIP returns true", func(t *testing.T) {
			err := runVerifyPIP(t, 3, commP, sectorId, pip)

			assert.NoError(t, err)
		})

		t.Run("PIP is invalid if miner hasn't committed sector", func(t *testing.T) {
			wrongSectorId := sectorId + 1
			err := runVerifyPIP(t, 3, commP, wrongSectorId, pip)

			require.Error(t, err)
			assert.Equal(t, "sector not committed", err.Error())
		})

		t.Run("PIP is valid if miner's PoSts are before the end of the grace period", func(t *testing.T) {
			blockHeight := blockheightOfPoSt + PieceInclusionGracePeriodBlocks - 1
			err := runVerifyPIP(t, blockHeight, commP, sectorId, pip)

			assert.NoError(t, err)
		})

		t.Run("PIP is valid if miner's PoSts are at the very end of the grace period", func(t *testing.T) {
			blockHeight := blockheightOfPoSt + PieceInclusionGracePeriodBlocks
			err := runVerifyPIP(t, blockHeight, commP, sectorId, pip)

			assert.NoError(t, err)
		})

		t.Run("PIP is invalid if miner's PoSts are after the end of the grace period", func(t *testing.T) {
			blockHeight := blockheightOfPoSt + PieceInclusionGracePeriodBlocks + 1
			err := runVerifyPIP(t, blockHeight, commP, sectorId, pip)

			require.Error(t, err)
			assert.Equal(t, "proofs out of date", err.Error())
		})

		t.Run("PIP is invalid if proof is invalid", func(t *testing.T) {
			wrongPIP := append([]byte{pip[0] + 1}, pip[1:]...)
			err := runVerifyPIP(t, 3, commP, sectorId, wrongPIP)

			require.Error(t, err)
			assert.Equal(t, "invalid inclusion proof", err.Error())
		})

		t.Run("Malformed PIP is a validation error", func(t *testing.T) {
			wrongPIP := pip[1:]
			err := runVerifyPIP(t, 3, commP, sectorId, wrongPIP)

			assert.Error(t, err)
			assert.Equal(t, "malformed inclusion proof", err.Error())
		})
	})
}

func TestGetProofsMode(t *testing.T) {
	ctx := context.Background()
	st, vms := core.CreateStorages(ctx, t)

	gasTracker := vm.NewGasTracker()
	gasTracker.MsgGasLimit = 99999

	t.Run("in TestMode", func(t *testing.T) {
		vmCtx := vm.NewVMContext(vm.NewContextParams{
			From:        &actor.Actor{},
			To:          &actor.Actor{},
			Message:     &types.Message{},
			State:       state.NewCachedStateTree(st),
			StorageMap:  vms,
			GasTracker:  gasTracker,
			BlockHeight: types.NewBlockHeight(0),
			Ancestors:   []types.TipSet{},
		})

		require.NoError(t, consensus.SetupDefaultActors(ctx, st, vms, types.TestProofsMode))

		mode, err := GetProofsMode(vmCtx)
		require.NoError(t, err)
		assert.Equal(t, types.TestProofsMode, mode)
	})

	t.Run("in LiveMode", func(t *testing.T) {
		vmCtx := vm.NewVMContext(vm.NewContextParams{
			From:        &actor.Actor{},
			To:          &actor.Actor{},
			Message:     &types.Message{},
			State:       state.NewCachedStateTree(st),
			StorageMap:  vms,
			GasTracker:  gasTracker,
			BlockHeight: types.NewBlockHeight(0),
			Ancestors:   []types.TipSet{},
		})

		require.NoError(t, consensus.SetupDefaultActors(ctx, st, vms, types.LiveProofsMode))

		mode, err := GetProofsMode(vmCtx)
		require.NoError(t, err)
		assert.Equal(t, types.LiveProofsMode, mode)
	})
}

func parseAbiBoolean(bytes []byte) bool {
	return bytes[0] == 1
}
