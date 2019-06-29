package miner_test

import (
	"context"
	"github.com/filecoin-project/go-filecoin/exec"
	"math/big"
	"testing"

	"github.com/libp2p/go-libp2p-peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

func TestAskFunctions(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := th.RequireCreateStorages(ctx, t)

	minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

	// make an ask, and then make sure it all looks good
	pdata := actor.MustConvertParams(types.NewAttoFILFromFIL(5), big.NewInt(1500))
	msg := types.NewMessage(address.TestAddress, minerAddr, 1, types.ZeroAttoFIL, "addAsk", pdata)

	_, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(1))
	assert.NoError(t, err)

	pdata = actor.MustConvertParams(big.NewInt(0))
	msg = types.NewMessage(address.TestAddress, minerAddr, 2, types.ZeroAttoFIL, "getAsk", pdata)
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
	msg = types.NewMessage(address.TestAddress, minerAddr, 2, types.ZeroAttoFIL, "getAsk", pdata)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(2))
	assert.Equal(t, Errors[ErrAskNotFound], result.ExecutionError)
	assert.NoError(t, err)

	// make another ask!
	pdata = actor.MustConvertParams(types.NewAttoFILFromFIL(110), big.NewInt(200))
	msg = types.NewMessage(address.TestAddress, minerAddr, 3, types.ZeroAttoFIL, "addAsk", pdata)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(3))
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(1), big.NewInt(0).SetBytes(result.Receipt.Return[0]))

	pdata = actor.MustConvertParams(big.NewInt(1))
	msg = types.NewMessage(address.TestAddress, minerAddr, 4, types.ZeroAttoFIL, "getAsk", pdata)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(4))
	assert.NoError(t, err)

	var ask2 Ask
	err = actor.UnmarshalStorage(result.Receipt.Return[0], &ask2)
	require.NoError(t, err)
	assert.Equal(t, types.NewBlockHeight(203), ask2.Expiry)
	assert.Equal(t, uint64(1), ask2.ID.Uint64())

	msg = types.NewMessage(address.TestAddress, minerAddr, 5, types.ZeroAttoFIL, "getAsks", nil)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(4))
	assert.NoError(t, err)
	assert.NoError(t, result.ExecutionError)

	var askids []uint64
	require.NoError(t, actor.UnmarshalStorage(result.Receipt.Return[0], &askids))
	assert.Len(t, askids, 2)
}

func TestChangeWorker(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := th.RequireCreateStorages(ctx, t)

	t.Run("Change worker address", func(t *testing.T) {
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		// retrieve worker before changing it
		result := callQueryMethodSuccess("getWorker", ctx, t, st, vms, address.TestAddress, minerAddr)
		addr := mustDeserializeAddress(t, result)

		assert.Equal(t, address.TestAddress, addr)

		// change worker
		pdata := actor.MustConvertParams(address.TestAddress2)
		msg := types.NewMessage(address.TestAddress, minerAddr, 1, types.ZeroAttoFIL, "changeWorker", pdata)

		_, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(1))
		assert.NoError(t, err)

		// retrieve worker
		result = callQueryMethodSuccess("getWorker", ctx, t, st, vms, address.TestAddress, minerAddr)
		addr = mustDeserializeAddress(t, result)

		assert.Equal(t, address.TestAddress2, addr)

		// ensure owner is not also changed
		result = callQueryMethodSuccess("getOwner", ctx, t, st, vms, address.TestAddress, minerAddr)
		addr = mustDeserializeAddress(t, result)

		assert.Equal(t, address.TestAddress, addr)
	})

	t.Run("Only owner can change address", func(t *testing.T) {
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		// change worker
		pdata := actor.MustConvertParams(address.TestAddress2)
		badActor := address.TestAddress2
		msg := types.NewMessage(badActor, minerAddr, 1, types.ZeroAttoFIL, "changeWorker", pdata)

		result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(1))
		assert.NoError(t, err)
		require.Error(t, result.ExecutionError)
		assert.Contains(t, result.ExecutionError.Error(), "not authorized")
		assert.Equal(t, uint8(ErrCallerUnauthorized), result.Receipt.ExitCode)
	})

	t.Run("Errors when gas cost too low", func(t *testing.T) {
		minerAddr := createTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))
		mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

		// change worker
		pdata := actor.MustConvertParams(address.TestAddress2)
		msg := types.NewMessage(mockSigner.Addresses[0], minerAddr, 0, types.ZeroAttoFIL, "changeWorker", pdata)

		gasPrice, _ := types.NewAttoFILFromFILString(".00001")
		gasLimit := types.NewGasUnits(10)
		result, err := th.ApplyTestMessageWithGas(st, vms, msg, types.NewBlockHeight(1), &mockSigner, gasPrice, gasLimit, mockSigner.Addresses[0])
		assert.NoError(t, err)

		require.Error(t, result.ExecutionError)
		assert.Contains(t, result.ExecutionError.Error(), "Insufficient gas")
		assert.Equal(t, uint8(exec.ErrInsufficientGas), result.Receipt.ExitCode)
	})
}

func TestGetWorker(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := th.RequireCreateStorages(ctx, t)

	minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

	// retrieve worker
	result := callQueryMethodSuccess("getWorker", ctx, t, st, vms, address.TestAddress, minerAddr)

	addrValue, err := abi.Deserialize(result[0], abi.Address)
	require.NoError(t, err)

	addr, ok := addrValue.Val.(address.Address)
	require.True(t, ok)

	assert.Equal(t, address.TestAddress, addr)
}

func TestGetOwner(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := th.RequireCreateStorages(ctx, t)

	minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

	// retrieve key
	result := callQueryMethodSuccess("getOwner", ctx, t, st, vms, address.TestAddress, minerAddr)

	addrValue, err := abi.Deserialize(result[0], abi.Address)
	require.NoError(t, err)

	addr, ok := addrValue.Val.(address.Address)
	require.True(t, ok)

	assert.Equal(t, address.TestAddress, addr)
}

func TestCBOREncodeState(t *testing.T) {
	tf.UnitTest(t)

	state := NewState(address.TestAddress, address.TestAddress, th.RequireRandomPeerID(t), types.OneKiBSectorSize)

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

		st, vms := th.RequireCreateStorages(ctx, t)

		origPid := th.RequireRandomPeerID(t)
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, origPid)

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

		st, vms := th.RequireCreateStorages(ctx, t)

		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		// update peer ID and expect authorization failure (TestAddress2 isn't the miner's worker address)
		updatePeerIdMsg := types.NewMessage(
			address.TestAddress2,
			minerAddr,
			th.RequireGetNonce(t, st, address.TestAddress2),
			types.NewAttoFILFromFIL(0),
			"updatePeerID",
			actor.MustConvertParams(th.RequireRandomPeerID(t)))

		applyMsgResult, err := th.ApplyTestMessage(st, vms, updatePeerIdMsg, types.NewBlockHeight(0))
		require.NoError(t, err)
		require.Equal(t, Errors[ErrCallerUnauthorized], applyMsgResult.ExecutionError)
		require.NotEqual(t, uint8(0), applyMsgResult.Receipt.ExitCode)
	})
}

func TestMinerGetPower(t *testing.T) {
	tf.UnitTest(t)

	t.Run("GetPower returns total storage committed to network", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := th.RequireCreateStorages(ctx, t)

		minerAddr := th.CreateTestMinerWith(types.NewAttoFILFromFIL(240), t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		// retrieve power (trivial result for no proven sectors)
		result := callQueryMethodSuccess("getPower", ctx, t, st, vms, address.TestAddress, minerAddr)
		require.True(t, types.ZeroBytes.Equal(types.NewBytesAmountFromBytes(result[0])))
	})
}

func TestMinerGetProvingPeriod(t *testing.T) {
	tf.UnitTest(t)

	t.Run("GetProvingPeriod returns unitialized values when proving period is unset", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := th.RequireCreateStorages(ctx, t)

		minerAddr := th.CreateTestMinerWith(types.NewAttoFILFromFIL(240), t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		// retrieve proving period
		result := callQueryMethodSuccess("getProvingPeriod", ctx, t, st, vms, address.TestAddress, minerAddr)
		startVal, err := abi.Deserialize(result[0], abi.BlockHeight)
		require.NoError(t, err)

		start, ok := startVal.Val.(*types.BlockHeight)
		require.True(t, ok)
		assert.Equal(t, types.NewBlockHeight(0), start)

		endVal, err := abi.Deserialize(result[0], abi.BlockHeight)
		require.NoError(t, err)

		end, ok := endVal.Val.(*types.BlockHeight)
		require.True(t, ok)
		assert.Equal(t, types.NewBlockHeight(0), end)
	})

	t.Run("GetProvingPeriod returns the start and end of the proving period", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := th.RequireCreateStorages(ctx, t)

		minerAddr := th.CreateTestMinerWith(types.NewAttoFILFromFIL(240), t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		// commit sector to set ProvingPeriodEnd
		commR := th.MakeCommitment()
		commRStar := th.MakeCommitment()
		commD := th.MakeCommitment()

		blockHeight := uint64(42)
		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, blockHeight, "commitSector", nil, uint64(1), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
		require.NoError(t, err)
		require.NoError(t, res.ExecutionError)
		require.Equal(t, uint8(0), res.Receipt.ExitCode)

		// retrieve proving period
		result := callQueryMethodSuccess("getProvingPeriod", ctx, t, st, vms, address.TestAddress, minerAddr)
		startVal, err := abi.Deserialize(result[0], abi.BlockHeight)
		require.NoError(t, err)

		start, ok := startVal.Val.(*types.BlockHeight)
		require.True(t, ok)
		assert.Equal(t, types.NewBlockHeight(42), start)

		endVal, err := abi.Deserialize(result[1], abi.BlockHeight)
		require.NoError(t, err)

		end, ok := endVal.Val.(*types.BlockHeight)
		require.True(t, ok)
		assert.Equal(t, types.NewBlockHeight(uint64(LargestSectorSizeProvingPeriodBlocks)+blockHeight), end)
	})
}

func updatePeerIdSuccess(ctx context.Context, t *testing.T, st state.Tree, vms vm.StorageMap, fromAddr address.Address, minerAddr address.Address, newPid peer.ID) {
	updatePeerIdMsg := types.NewMessage(
		fromAddr,
		minerAddr,
		th.RequireGetNonce(t, st, fromAddr),
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

	t.Run("a commitSector message is rejected if miner can't cover the required collateral", func(t *testing.T) {
		ctx := context.Background()
		st, vms := th.RequireCreateStorages(ctx, t)

		numSectorsToPledge := uint64(10)
		amtCollateralForPledge := MinimumCollateralPerSector.CalculatePrice(types.NewBytesAmount(numSectorsToPledge))

		origPid := th.RequireRandomPeerID(t)
		minerAddr := th.CreateTestMinerWith(amtCollateralForPledge, t, st, vms, address.TestAddress, origPid)

		commR := th.MakeCommitment()
		commRStar := th.MakeCommitment()
		commD := th.MakeCommitment()

		f := func(sectorId uint64) (*consensus.ApplicationResult, error) {
			return th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, "commitSector", nil, uint64(sectorId), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
		}

		// these commitments should exhaust miner's FIL
		for i := uint64(0); i < numSectorsToPledge; i++ {
			res, err := f(i)
			require.NoError(t, err)
			require.NoError(t, res.ExecutionError)
			require.Equal(t, uint8(0), res.Receipt.ExitCode)
		}

		// this commitment should be rejected (miner has no remaining FIL)
		res, err := f(numSectorsToPledge)
		require.NoError(t, err)
		require.Error(t, res.ExecutionError)
		require.NotEqual(t, uint8(0), res.Receipt.ExitCode)
	})

	t.Run("a miner successfully commits a sector", func(t *testing.T) {
		ctx := context.Background()
		st, vms := th.RequireCreateStorages(ctx, t)

		origPid := th.RequireRandomPeerID(t)
		minerAddr := th.CreateTestMinerWith(types.NewAttoFILFromFIL(100), t, st, vms, address.TestAddress, origPid)

		commR := th.MakeCommitment()
		commRStar := th.MakeCommitment()
		commD := th.MakeCommitment()

		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, "commitSector", nil, uint64(1), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
		require.NoError(t, err)
		require.NoError(t, res.ExecutionError)
		require.Equal(t, uint8(0), res.Receipt.ExitCode)

		// check that the proving period matches
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, "getProvingPeriod", nil)
		require.NoError(t, err)
		require.NoError(t, res.ExecutionError)

		// provingPeriodEnd is block height plus proving period
		provingPeriod := ProvingPeriodDuration(types.OneKiBSectorSize)

		// blockheight was 3
		require.Equal(t, types.NewBlockHeight(3+provingPeriod), types.NewBlockHeightFromBytes(res.Receipt.Return[1]))

		// fail because commR already exists
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 4, "commitSector", nil, uint64(1), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
		require.NoError(t, err)
		require.EqualError(t, res.ExecutionError, "sector already committed")
		require.Equal(t, uint8(0x23), res.Receipt.ExitCode)
	})
}

func TestMinerSubmitPoSt(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	st, vms := th.RequireCreateStorages(ctx, t)

	ancestors := th.RequireTipSetChain(t, 10)
	origPid := th.RequireRandomPeerID(t)
	minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, origPid)
	proof := th.MakeRandomPoStProofForTest()

	miner := state.MustGetActor(st, minerAddr)
	minerBalance := miner.Balance
	owner := state.MustGetActor(st, address.TestAddress)
	ownerBalance := owner.Balance

	firstCommitBlockHeight := uint64(3)
	secondProvingPeriodStart := LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight
	secondProvingPeriodEnd := 2*LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight
	lastPossibleSubmission := secondProvingPeriodStart + LargestSectorSizeProvingPeriodBlocks + LargestSectorGenerationAttackThresholdBlocks

	// add a sector
	res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight, "commitSector", ancestors, uint64(1), th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)

	// add another sector
	res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight+1, "commitSector", ancestors, uint64(2), th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)

	t.Run("on-time PoSt succeeds", func(t *testing.T) {
		// submit post
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight+5, "submitPoSt", ancestors, []types.PoStProof{proof})
		assert.NoError(t, err)
		assert.NoError(t, res.ExecutionError)
		assert.Equal(t, uint8(0), res.Receipt.ExitCode)

		// check that the proving period is now the next one
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight+6, "getProvingPeriod", ancestors)
		assert.NoError(t, err)
		assert.NoError(t, res.ExecutionError)
		assert.Equal(t, types.NewBlockHeightFromBytes(res.Receipt.Return[1]), types.NewBlockHeight(secondProvingPeriodEnd))
	})

	t.Run("after generation attack grace period rejected", func(t *testing.T) {
		// Rejected one block late
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission+1, "submitPoSt", ancestors, []types.PoStProof{proof})
		assert.NoError(t, err)
		assert.Error(t, res.ExecutionError)
	})

	t.Run("late submission charged fee", func(t *testing.T) {
		// Rejected on the deadline with message value not carrying sufficient fees
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission, "submitPoSt", ancestors, []types.PoStProof{proof})
		assert.NoError(t, err)
		assert.Error(t, res.ExecutionError)

		// Accepted on the deadline with a fee
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 1, lastPossibleSubmission, "submitPoSt", ancestors, []types.PoStProof{proof})
		assert.NoError(t, err)
		assert.NoError(t, res.ExecutionError)
		assert.Equal(t, uint8(0), res.Receipt.ExitCode)

		// Check miner's balance unchanged (because it's topped up from message value then fee burnt).
		postedCollateral := MinimumCollateralPerSector.Add(MinimumCollateralPerSector) // 2 sectors
		fee := LatePoStFee(postedCollateral, bh(secondProvingPeriodStart+LargestSectorSizeProvingPeriodBlocks), bh(lastPossibleSubmission), bh(LargestSectorGenerationAttackThresholdBlocks))
		miner := state.MustGetActor(st, minerAddr)
		assert.Equal(t, minerBalance.String(), miner.Balance.String())

		// Check  change was refunded to owner, balance is now reduced by fee.
		owner, err := st.GetActor(ctx, address.TestAddress)
		assert.NoError(t, err)
		assert.Equal(t, ownerBalance.Sub(fee).String(), owner.Balance.String())
	})
}

func TestVerifyPIP(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	st, vms := th.RequireCreateStorages(ctx, t)

	ancestors := th.RequireTipSetChain(t, 10)

	origPid := th.RequireRandomPeerID(t)
	minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, origPid)

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
		proof := th.MakeRandomPoStProofForTest()
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
	st, vms := th.RequireCreateStorages(ctx, t)

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

func TestLatePoStFee(t *testing.T) {
	pledgeCollateral := af(1000)

	t.Run("on time charges no fee", func(t *testing.T) {
		assert.True(t, types.ZeroAttoFIL.Equal(LatePoStFee(pledgeCollateral, bh(1000), bh(999), bh(100))))
		assert.True(t, types.ZeroAttoFIL.Equal(LatePoStFee(pledgeCollateral, bh(1000), bh(1000), bh(100))))
	})

	t.Run("fee proportional to lateness", func(t *testing.T) {
		// 1 block late is 1% of 100 allowable
		assert.True(t, af(10).Equal(LatePoStFee(pledgeCollateral, bh(1000), bh(1001), bh(100))))
		// 5 blocks late of 100 allowable
		assert.True(t, af(50).Equal(LatePoStFee(pledgeCollateral, bh(1000), bh(1005), bh(100))))

		// 2 blocks late of 10000 allowable, fee rounds down to zero
		assert.True(t, af(0).Equal(LatePoStFee(pledgeCollateral, bh(1000), bh(1002), bh(10000))))
		// 9 blocks late of 10000 allowable, fee rounds down to zero
		assert.True(t, af(0).Equal(LatePoStFee(pledgeCollateral, bh(1000), bh(1009), bh(10000))))
		// 10 blocks late of 10000 allowable is 1/1000
		assert.True(t, af(1).Equal(LatePoStFee(pledgeCollateral, bh(1000), bh(1010), bh(10000))))
	})

	t.Run("fee capped at total pledge", func(t *testing.T) {
		assert.True(t, pledgeCollateral.Equal(LatePoStFee(pledgeCollateral, bh(1000), bh(1100), bh(100))))
		assert.True(t, pledgeCollateral.Equal(LatePoStFee(pledgeCollateral, bh(1000), bh(2000), bh(100))))
	})
}

func mustDeserializeAddress(t *testing.T, result [][]byte) address.Address {
	addrValue, err := abi.Deserialize(result[0], abi.Address)
	require.NoError(t, err)

	addr, ok := addrValue.Val.(address.Address)
	require.True(t, ok)

	return addr
}

func af(h int64) types.AttoFIL {
	return types.NewAttoFIL(big.NewInt(h))
}

func bh(h uint64) *types.BlockHeight {
	return types.NewBlockHeight(uint64(h))
}
