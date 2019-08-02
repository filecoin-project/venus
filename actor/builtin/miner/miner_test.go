package miner_test

import (
	"context"
	"errors"
	"math/big"
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	vmerrors "github.com/filecoin-project/go-filecoin/vm/errors"
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
	assert.NoError(t, err)
	assert.Equal(t, Errors[ErrAskNotFound], result.ExecutionError)

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
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))
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

func TestGetActiveCollateral(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := th.RequireCreateStorages(ctx, t)

	minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

	// Check 0 collateral
	result := callQueryMethodSuccess("getActiveCollateral", ctx, t, st, vms, address.TestAddress, minerAddr)
	attoFILValue, err := abi.Deserialize(result[0], abi.AttoFIL)
	require.NoError(t, err)

	coll, ok := attoFILValue.Val.(types.AttoFIL)
	require.True(t, ok)

	assert.Equal(t, types.ZeroAttoFIL, coll)

	// Commit a sector.
	commR := th.MakeCommitment()
	commRStar := th.MakeCommitment()
	commD := th.MakeCommitment()

	res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, "commitSector", nil, uint64(0), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)

	// Check updated collateral
	result = callQueryMethodSuccess("getActiveCollateral", ctx, t, st, vms, address.TestAddress, minerAddr)
	attoFILValue, err = abi.Deserialize(result[0], abi.AttoFIL)
	require.NoError(t, err)

	coll, ok = attoFILValue.Val.(types.AttoFIL)
	require.True(t, ok)

	assert.Equal(t, MinimumCollateralPerSector, coll)
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
		updatePeerIdSuccess(t, st, vms, address.TestAddress, minerAddr, newPid)

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

		minerAddr := th.CreateTestMinerWith(types.NewAttoFILFromFIL(240), t, st, vms, address.TestAddress, th.RequireRandomPeerID(t), 0)

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

		minerAddr := th.CreateTestMinerWith(types.NewAttoFILFromFIL(240), t, st, vms, address.TestAddress, th.RequireRandomPeerID(t), 0)

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

		minerAddr := th.CreateTestMinerWith(types.NewAttoFILFromFIL(240), t, st, vms, address.TestAddress, th.RequireRandomPeerID(t), 0)

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

func updatePeerIdSuccess(t *testing.T, st state.Tree, vms vm.StorageMap, fromAddr address.Address, minerAddr address.Address, newPid peer.ID) {
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
		minerAddr := th.CreateTestMinerWith(amtCollateralForPledge, t, st, vms, address.TestAddress, origPid, 0)

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
		minerAddr := th.CreateTestMinerWith(types.NewAttoFILFromFIL(100), t, st, vms, address.TestAddress, origPid, 0)

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
		require.EqualError(t, res.ExecutionError, "sector already committed at this ID")
		require.Equal(t, uint8(0x23), res.Receipt.ExitCode)
	})
}

// minerActorLiason provides a set of test friendly calls for setting up, reading
// internals, and transitioning the portion of the filecoin state machine
// related to a particular miner actor.
type minerActorLiason struct {
	st            state.Tree
	vms           vm.StorageMap
	ancestors     []types.TipSet
	minerAddr     address.Address
	t             *testing.T
	currentHeight uint64
}

func (mal *minerActorLiason) requireHeightNotPast(blockHeight uint64) {
	require.True(mal.t, blockHeight >= mal.currentHeight)
	mal.currentHeight = blockHeight
}

func (mal *minerActorLiason) requireCommit(blockHeight, sectorID uint64) {
	mal.requireHeightNotPast(blockHeight)
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, mal.minerAddr, 0, blockHeight, "commitSector", mal.ancestors, sectorID, th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(mal.t, err)
	require.NoError(mal.t, res.ExecutionError)
	require.Equal(mal.t, uint8(0), res.Receipt.ExitCode)
}

func (mal *minerActorLiason) requirePoSt(blockHeight uint64, done types.IntSet, faults types.FaultSet) {
	mal.requireHeightNotPast(blockHeight)
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, mal.minerAddr, 0, blockHeight, "submitPoSt", mal.ancestors, []types.PoStProof{th.MakeRandomPoStProofForTest()}, faults, done)
	assert.NoError(mal.t, err)
	assert.NoError(mal.t, res.ExecutionError)
	assert.Equal(mal.t, uint8(0), res.Receipt.ExitCode)
}

func (mal *minerActorLiason) requireReadState() State {
	miner := state.MustGetActor(mal.st, mal.minerAddr)
	storage := mal.vms.NewStorage(mal.minerAddr, miner)
	stateBytes, err := storage.Get(storage.Head())
	require.NoError(mal.t, err)
	var minerState State
	err = actor.UnmarshalStorage(stateBytes, &minerState)
	require.NoError(mal.t, err)
	return minerState
}

func (mal *minerActorLiason) requirePower(queryHeight uint64) *types.BytesAmount {
	mal.requireHeightNotPast(queryHeight)
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, mal.minerAddr, 0, queryHeight, "getPower", mal.ancestors)
	require.NoError(mal.t, err)
	require.NoError(mal.t, res.ExecutionError)
	require.Equal(mal.t, uint8(0), res.Receipt.ExitCode)
	require.Equal(mal.t, 1, len(res.Receipt.Return))
	return types.NewBytesAmountFromBytes(res.Receipt.Return[0])
}

func (mal *minerActorLiason) requireTotalStorage(queryHeight uint64) *types.BytesAmount {
	mal.requireHeightNotPast(queryHeight)
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, address.StorageMarketAddress, 0, queryHeight, "getTotalStorage", mal.ancestors)
	require.NoError(mal.t, err)
	require.NoError(mal.t, res.ExecutionError)
	require.Equal(mal.t, uint8(0), res.Receipt.ExitCode)
	require.Equal(mal.t, 1, len(res.Receipt.Return))
	return types.NewBytesAmountFromBytes(res.Receipt.Return[0])
}

func (mal *minerActorLiason) assertPoStFail(blockHeight uint64, done types.IntSet, exitCode uint8) {
	mal.requireHeightNotPast(blockHeight)
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, mal.minerAddr, 0, blockHeight, "submitPoSt", mal.ancestors, []types.PoStProof{th.MakeRandomPoStProofForTest()}, types.EmptyFaultSet(), done)
	assert.NoError(mal.t, err)
	assert.Error(mal.t, res.ExecutionError)
	assert.Equal(mal.t, exitCode, res.Receipt.ExitCode)
}

func (mal *minerActorLiason) assertPoStStateAtHeight(expected int64, queryHeight uint64) {
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, mal.minerAddr, 0, queryHeight, "getPoStState", mal.ancestors)
	assert.NoError(mal.t, err)
	require.NotNil(mal.t, res)

	ret, err := abi.Deserialize(res.Receipt.Return[0], abi.Integer)
	require.NoError(mal.t, err)

	assert.Equal(mal.t, big.NewInt(expected), ret.Val)
}

func newMinerActorLiason(t *testing.T, st state.Tree, vms vm.StorageMap, ancestors []types.TipSet, minerAddr address.Address) *minerActorLiason {
	return &minerActorLiason{
		t:             t,
		st:            st,
		vms:           vms,
		ancestors:     ancestors,
		minerAddr:     minerAddr,
		currentHeight: 0,
	}
}

func setupMinerActorLiason(t *testing.T) *minerActorLiason {
	ctx := context.Background()
	st, vms := th.RequireCreateStorages(ctx, t)

	builder := chain.NewBuilder(t, address.Undef)
	head := builder.AppendManyOn(10, types.UndefTipSet)
	ancestors := builder.RequireTipSets(head.Key(), 10)
	origPid := th.RequireRandomPeerID(t)
	minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, origPid)
	return newMinerActorLiason(t, st, vms, ancestors, minerAddr)
}

func TestMinerSubmitPoStPowerUpdates(t *testing.T) {
	tf.UnitTest(t)

	firstCommitBlockHeight := uint64(3)
	secondProvingPeriodStart := LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight
	thirdProvingPeriodStart := 2*LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight
	fourthProvingPeriodStart := 3*LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight

	faults := types.EmptyFaultSet()

	t.Run("power is 0 until first PoSt", func(t *testing.T) {
		mal := setupMinerActorLiason(t)

		// commit several sectors
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(3))

		power := mal.requirePower(firstCommitBlockHeight + 2)
		assert.Equal(t, types.NewBytesAmount(0), power)
	})

	t.Run("power is 1 after first PoSt", func(t *testing.T) {
		mal := setupMinerActorLiason(t)

		// commit several sectors
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		mal.requireCommit(firstCommitBlockHeight+2, uint64(3))

		// submit PoSt and add some power.
		done := types.EmptyIntSet()
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)

		power := mal.requirePower(firstCommitBlockHeight + 5)
		assert.Equal(t, types.OneKiBSectorSize, power)
	})

	t.Run("power accumulates over multiple proving periods", func(t *testing.T) {
		mal := setupMinerActorLiason(t)

		// Period 1 commit and prove
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		done := types.EmptyIntSet()
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)
		power := mal.requirePower(firstCommitBlockHeight + 6)
		assert.Equal(t, types.OneKiBSectorSize, power)

		// Period 2 commit and prove
		mal.requireCommit(secondProvingPeriodStart+1, uint64(16))
		mal.requireCommit(secondProvingPeriodStart+2, uint64(17))
		mal.requirePoSt(secondProvingPeriodStart+5, done, faults)
		power = mal.requirePower(secondProvingPeriodStart + 6)
		assert.Equal(t, types.NewBytesAmount(2).Mul(types.OneKiBSectorSize), power)

		// Period 3 prove over 4 sectors and measure power
		mal.requirePoSt(thirdProvingPeriodStart+5, done, faults)
		power = mal.requirePower(thirdProvingPeriodStart + 6)
		assert.Equal(t, types.NewBytesAmount(4).Mul(types.OneKiBSectorSize), power)
	})

	t.Run("power removed with sectors", func(t *testing.T) {
		mal := setupMinerActorLiason(t)

		// Period 1 commit and prove
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		done := types.EmptyIntSet()
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)

		// Period 2 commit and prove
		mal.requireCommit(secondProvingPeriodStart+1, uint64(16))
		mal.requireCommit(secondProvingPeriodStart+2, uint64(17))
		mal.requirePoSt(secondProvingPeriodStart+5, done, faults)

		// Period 3 prove and drop 1 and 2
		done = types.NewIntSet(1, 2)
		mal.requirePoSt(thirdProvingPeriodStart+5, done, faults)

		// power lags removal by a proving period
		power := mal.requirePower(thirdProvingPeriodStart + 6)
		assert.Equal(t, types.NewBytesAmount(4).Mul(types.OneKiBSectorSize), power)

		// next period power is removed
		done = types.EmptyIntSet()
		mal.requirePoSt(fourthProvingPeriodStart+1, done, faults)
		power = mal.requirePower(fourthProvingPeriodStart + 2)
		assert.Equal(t, types.NewBytesAmount(2).Mul(types.OneKiBSectorSize), power)
	})

	t.Run("faults removes power and sector commitments", func(t *testing.T) {
		mal := setupMinerActorLiason(t)
		done := types.EmptyIntSet()

		// commit several sectors and PoSt
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		mal.requireCommit(firstCommitBlockHeight+2, uint64(3))
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)

		// Second proving period
		mal.requirePoSt(secondProvingPeriodStart, done, faults)
		power := mal.requirePower(secondProvingPeriodStart + 1)
		assert.Equal(t, types.NewBytesAmount(3).Mul(types.OneKiBSectorSize), power)

		// PoSt with some faults and check that power has decreased
		mal.requirePoSt(thirdProvingPeriodStart, done, types.NewFaultSet([]uint64{1, 2}))
		power = mal.requirePower(thirdProvingPeriodStart + 1)
		assert.Equal(t, types.NewBytesAmount(1).Mul(types.OneKiBSectorSize), power)

		// Ensure that sector commitments have been updated
		state := mal.requireReadState()
		assert.False(t, state.SectorCommitments.Has(uint64(1)))
		assert.False(t, state.SectorCommitments.Has(uint64(2)))
		assert.True(t, state.SectorCommitments.Has(uint64(3)))
	})
}

func TestMinerSubmitPoStVerification(t *testing.T) {
	tf.UnitTest(t)

	message := types.NewMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, "submitPoSt", nil)
	comm1 := th.MakeCommitments()
	comm2 := th.MakeCommitments()
	comm3 := th.MakeCommitments()

	t.Run("Sends correct parameters to post verifier", func(t *testing.T) {
		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(ProvingPeriodDuration(types.OneKiBSectorSize))
		minerState.SectorCommitments = NewSectorSet()
		minerState.SectorCommitments.Add(1, types.Commitments{CommR: comm1.CommR})
		minerState.SectorCommitments.Add(2, types.Commitments{CommR: comm2.CommR})
		minerState.SectorCommitments.Add(3, types.Commitments{CommR: comm3.CommR})

		// The 3 sector is not in the proving set, so its CommR should not appear in the VerifyPoSt request
		minerState.ProvingSet = types.NewIntSet(1, 2)
		verifier := &verification.FakeVerifier{
			VerifyPoStValid: true,
		}
		vmctx := th.NewFakeVMContextWithVerifier(message, minerState, verifier)

		miner := Actor{Bootstrap: false}

		testProof := []types.PoStProof{th.MakeRandomPoStProofForTest()}
		_, err := miner.SubmitPoSt(vmctx, testProof, types.EmptyFaultSet(), types.EmptyIntSet())
		require.NoError(t, err)

		require.NotNil(t, verifier.LastReceivedVerifyPoStRequest)
		assert.Equal(t, types.OneKiBSectorSize, verifier.LastReceivedVerifyPoStRequest.SectorSize)

		seed := types.PoStChallengeSeed{}
		copy(seed[:], vmctx.RandomnessValue)

		sortedRs := proofs.NewSortedCommRs(comm1.CommR, comm2.CommR)

		assert.Equal(t, seed, verifier.LastReceivedVerifyPoStRequest.ChallengeSeed)
		assert.Equal(t, 0, len(verifier.LastReceivedVerifyPoStRequest.Faults))
		assert.Equal(t, 1, len(verifier.LastReceivedVerifyPoStRequest.Proofs))
		assert.Equal(t, testProof, verifier.LastReceivedVerifyPoStRequest.Proofs)
		assert.Equal(t, 2, len(verifier.LastReceivedVerifyPoStRequest.SortedCommRs.Values()))
		assert.Equal(t, sortedRs.Values()[0], verifier.LastReceivedVerifyPoStRequest.SortedCommRs.Values()[0])
		assert.Equal(t, sortedRs.Values()[1], verifier.LastReceivedVerifyPoStRequest.SortedCommRs.Values()[1])
	})

	t.Run("Faults if proving set commitment is missing from sector commitments", func(t *testing.T) {
		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(ProvingPeriodDuration(types.OneKiBSectorSize))
		minerState.SectorCommitments = NewSectorSet()

		minerState.ProvingSet = types.NewIntSet(4)
		vmctx := th.NewFakeVMContext(message, minerState)

		miner := Actor{Bootstrap: false}

		testProof := []types.PoStProof{th.MakeRandomPoStProofForTest()}
		code, err := miner.SubmitPoSt(vmctx, testProof, types.EmptyFaultSet(), types.NewIntSet())
		require.Error(t, err)
		assert.Equal(t, "miner ProvingSet sector id 4 missing in SectorCommitments", err.Error())
		assert.True(t, vmerrors.IsFault(err))
		assert.Equal(t, uint8(1), code)
	})

	t.Run("Reverts if verification errors", func(t *testing.T) {
		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(ProvingPeriodDuration(types.OneKiBSectorSize))
		minerState.SectorCommitments = NewSectorSet()
		minerState.SectorCommitments.Add(1, types.Commitments{CommR: comm1.CommR})

		minerState.ProvingSet = types.NewIntSet(1)
		verifier := &verification.FakeVerifier{
			VerifyPoStError: errors.New("verifier error"),
		}

		vmctx := th.NewFakeVMContextWithVerifier(message, minerState, verifier)

		miner := Actor{Bootstrap: false}

		testProof := []types.PoStProof{th.MakeRandomPoStProofForTest()}
		code, err := miner.SubmitPoSt(vmctx, testProof, types.EmptyFaultSet(), types.NewIntSet())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "verifier error")
		assert.True(t, vmerrors.ShouldRevert(err))
		assert.Equal(t, uint8(1), code)
	})

	t.Run("Reverts if proof is invalid", func(t *testing.T) {
		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(ProvingPeriodDuration(types.OneKiBSectorSize))
		minerState.SectorCommitments = NewSectorSet()
		minerState.SectorCommitments.Add(1, types.Commitments{CommR: comm1.CommR})

		minerState.ProvingSet = types.NewIntSet(1)
		verifier := &verification.FakeVerifier{
			VerifyPoStValid: false,
		}

		vmctx := th.NewFakeVMContextWithVerifier(message, minerState, verifier)

		miner := Actor{Bootstrap: false}

		testProof := []types.PoStProof{th.MakeRandomPoStProofForTest()}
		code, err := miner.SubmitPoSt(vmctx, testProof, types.EmptyFaultSet(), types.NewIntSet())
		require.Error(t, err)
		assert.Equal(t, Errors[ErrInvalidPoSt], err)
		assert.True(t, vmerrors.ShouldRevert(err))
		assert.Equal(t, uint8(ErrInvalidPoSt), code)
	})
}

func TestMinerSubmitPoStProvingSet(t *testing.T) {
	tf.UnitTest(t)

	firstCommitBlockHeight := uint64(3)
	secondProvingPeriodStart := LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight
	thirdProvingPeriodStart := 2*LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight

	faults := types.EmptyFaultSet()

	t.Run("empty proving set before first commit", func(t *testing.T) {
		mal := setupMinerActorLiason(t)
		mSt := mal.requireReadState()
		assert.Equal(t, types.EmptyIntSet().Values(), mSt.ProvingSet.Values())
	})

	t.Run("only one sector added to proving set during first period", func(t *testing.T) {
		mal := setupMinerActorLiason(t)
		// commit several sectors
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight, uint64(2))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(3))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(4))
		mal.requireCommit(firstCommitBlockHeight+2, uint64(16))
		mal.requireCommit(firstCommitBlockHeight+2, uint64(17))

		mSt := mal.requireReadState()
		assert.Equal(t, types.NewIntSet(1).Values(), mSt.ProvingSet.Values())
	})

	t.Run("all committed sectors added to proving set after PoSt submission", func(t *testing.T) {
		mal := setupMinerActorLiason(t)

		// commit several sectors
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		mal.requireCommit(firstCommitBlockHeight+2, uint64(16))
		mal.requireCommit(firstCommitBlockHeight+2, uint64(17))

		// submit PoSt to update proving set
		done := types.EmptyIntSet()
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)

		mSt := mal.requireReadState()
		assert.Equal(t, types.NewIntSet(1, 2, 16, 17).Values(), mSt.ProvingSet.Values())
	})

	t.Run("committed sectors acrue across multiple PoSt submissions", func(t *testing.T) {
		mal := setupMinerActorLiason(t)

		// Period 1 commit and prove
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		done := types.EmptyIntSet()
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)

		// Period 2 commit and prove
		mal.requireCommit(secondProvingPeriodStart+1, uint64(16))
		mal.requireCommit(secondProvingPeriodStart+2, uint64(17))
		mal.requirePoSt(secondProvingPeriodStart+5, done, faults)

		// Period 3 commit and prove
		mal.requireCommit(thirdProvingPeriodStart+1, uint64(4))
		mal.requirePoSt(thirdProvingPeriodStart+5, done, faults)

		mSt := mal.requireReadState()
		assert.Equal(t, types.NewIntSet(1, 2, 4, 16, 17).Values(), mSt.ProvingSet.Values())
	})

	t.Run("done sectors removed from proving set", func(t *testing.T) {
		mal := setupMinerActorLiason(t)

		// commit several sectors
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		mal.requireCommit(firstCommitBlockHeight+2, uint64(17))

		// submit PoSt to update proving set and remove sector 17
		done := types.NewIntSet(17)
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)
		mSt := mal.requireReadState()
		assert.Equal(t, types.NewIntSet(1, 2).Values(), mSt.ProvingSet.Values())
	})

}

func TestMinerSubmitPoStNextDoneSet(t *testing.T) {
	tf.UnitTest(t)

	firstCommitBlockHeight := uint64(3)
	secondProvingPeriodStart := LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight
	thirdProvingPeriodStart := 2*LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight

	faults := types.EmptyFaultSet()

	t.Run("next done set empty when done arg empty", func(t *testing.T) {
		mal := setupMinerActorLiason(t)
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		mal.requireCommit(firstCommitBlockHeight+2, uint64(17))

		// submit PoSt to update proving set with no done sectors
		done := types.EmptyIntSet()
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)
		mSt := mal.requireReadState()
		assert.Equal(t, types.EmptyIntSet().Values(), mSt.NextDoneSet.Values())
	})

	t.Run("next done set updates when sectors completed", func(t *testing.T) {
		mal := setupMinerActorLiason(t)

		// Period 1 commit and prove
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(3))
		done := types.EmptyIntSet()
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)

		// Period 2 remove id 2 and 3
		done = types.NewIntSet(2, 3)
		mal.requirePoSt(secondProvingPeriodStart+5, done, faults)
		mSt := mal.requireReadState()
		assert.Equal(t, done.Values(), mSt.NextDoneSet.Values())
	})

	t.Run("next done set resets after additional post", func(t *testing.T) {
		mal := setupMinerActorLiason(t)

		// Period 1 commit and prove
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(3))
		done := types.EmptyIntSet()
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)

		// Period 2 remove id 2 and 3
		done = types.NewIntSet(2, 3)
		mal.requirePoSt(secondProvingPeriodStart+5, done, faults)

		// Period 3 commit and prove
		done = types.EmptyIntSet()
		mal.requirePoSt(thirdProvingPeriodStart+5, done, faults)

		mSt := mal.requireReadState()
		assert.Equal(t, types.EmptyIntSet().Values(), mSt.NextDoneSet.Values())

	})

	t.Run("submitPoSt fails if miner does not have done ids stored", func(t *testing.T) {
		mal := setupMinerActorLiason(t)

		// Period 1 commit and prove
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(3))
		done := types.EmptyIntSet()
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)

		failingDone := done.Add(uint64(30))
		mal.assertPoStFail(secondProvingPeriodStart+5, failingDone, uint8(ErrInvalidSector))
	})
}

func TestMinerSubmitPoSt(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	st, vms := th.RequireCreateStorages(ctx, t)

	builder := chain.NewBuilder(t, address.Undef)
	head := builder.AppendManyOn(10, types.UndefTipSet)
	ancestors := builder.RequireTipSets(head.Key(), 10)
	origPid := th.RequireRandomPeerID(t)
	minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, origPid)
	proof := th.MakeRandomPoStProofForTest()
	doneDefault := types.EmptyIntSet()
	faultsDefault := types.EmptyFaultSet()

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
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight+5, "submitPoSt", ancestors, []types.PoStProof{proof}, faultsDefault, doneDefault)
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
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission+1, "submitPoSt", ancestors, []types.PoStProof{proof}, faultsDefault, doneDefault)
		assert.NoError(t, err)
		assert.Error(t, res.ExecutionError)
	})

	t.Run("late submission charged fee", func(t *testing.T) {
		// Rejected on the deadline with message value not carrying sufficient fees
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission, "submitPoSt", ancestors, []types.PoStProof{proof}, faultsDefault, doneDefault)
		assert.NoError(t, err)
		assert.Error(t, res.ExecutionError)

		// Accepted on the deadline with a fee
		// Must calculate fee before submitting the PoSt, since submission will reset the proving period.
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission, "calculateLateFee", ancestors, lastPossibleSubmission)
		fee := types.NewAttoFILFromBytes(res.Receipt.Return[0])
		require.False(t, fee.IsZero())

		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 1, lastPossibleSubmission, "submitPoSt", ancestors, []types.PoStProof{proof}, faultsDefault, doneDefault)
		assert.NoError(t, err)
		assert.NoError(t, res.ExecutionError)
		assert.Equal(t, uint8(0), res.Receipt.ExitCode)

		// Check miner's balance unchanged (because it's topped up from message value then fee burnt).
		miner := state.MustGetActor(st, minerAddr)
		assert.Equal(t, minerBalance.String(), miner.Balance.String())

		// Check  change was refunded to owner, balance is now reduced by fee.
		owner, err := st.GetActor(ctx, address.TestAddress)
		assert.NoError(t, err)
		assert.Equal(t, ownerBalance.Sub(fee).String(), owner.Balance.String())
	})
}

func TestActorSlashStorageFault(t *testing.T) {
	tf.UnitTest(t)

	firstCommitBlockHeight := uint64(3)
	secondProvingPeriodStart := firstCommitBlockHeight + ProvingPeriodDuration(types.OneKiBSectorSize)
	thirdProvingPeriodStart := secondProvingPeriodStart + ProvingPeriodDuration(types.OneKiBSectorSize)
	thirdProvingPeriodEnd := thirdProvingPeriodStart + ProvingPeriodDuration(types.OneKiBSectorSize)
	lastPossibleSubmission := thirdProvingPeriodEnd + LargestSectorGenerationAttackThresholdBlocks

	// CreateTestMiner creates a new test miner with the given peerID and miner
	// owner address and a given number of committed sectors
	createMinerWithPower := func(t *testing.T) (state.Tree, vm.StorageMap, address.Address) {
		ctx := context.Background()
		st, vms := th.RequireCreateStorages(ctx, t)
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		builder := chain.NewBuilder(t, address.Undef)
		head := builder.AppendManyOn(10, types.UndefTipSet)
		ancestors := builder.RequireTipSets(head.Key(), 10)
		proof := th.MakeRandomPoStProofForTest()
		doneDefault := types.EmptyIntSet()
		faultsDefault := types.EmptyFaultSet()

		// add a sector
		_, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight, "commitSector", ancestors, uint64(1), th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
		require.NoError(t, err)

		// add another sector (not in proving set yet)
		_, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight+1, "commitSector", ancestors, uint64(2), th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
		require.NoError(t, err)

		// submit post (first sector only)
		_, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, secondProvingPeriodStart, "submitPoSt", ancestors, []types.PoStProof{proof}, faultsDefault, doneDefault)
		require.NoError(t, err)

		// submit post (both sectors
		_, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, thirdProvingPeriodStart, "submitPoSt", ancestors, []types.PoStProof{proof}, faultsDefault, doneDefault)
		assert.NoError(t, err)

		return st, vms, minerAddr
	}

	t.Run("slashing charges gas", func(t *testing.T) {
		st, vms, minerAddr := createMinerWithPower(t)
		mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

		// change worker
		msg := types.NewMessage(mockSigner.Addresses[0], minerAddr, 0, types.ZeroAttoFIL, "slashStorageFault", []byte{})

		gasPrice, _ := types.NewAttoFILFromFILString(".00001")
		gasLimit := types.NewGasUnits(10)
		result, err := th.ApplyTestMessageWithGas(st, vms, msg, types.NewBlockHeight(1), &mockSigner, gasPrice, gasLimit, mockSigner.Addresses[0])
		require.NoError(t, err)

		require.Error(t, result.ExecutionError)
		assert.Contains(t, result.ExecutionError.Error(), "Insufficient gas")
		assert.Equal(t, uint8(exec.ErrInsufficientGas), result.Receipt.ExitCode)
	})

	t.Run("slashing a miner with no storage fails", func(t *testing.T) {
		ctx := context.Background()
		st, vms := th.RequireCreateStorages(ctx, t)
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission+1, "slashStorageFault", nil)
		require.NoError(t, err)
		assert.Contains(t, res.ExecutionError.Error(), "miner is inactive")
		assert.Equal(t, uint8(ErrMinerNotSlashable), res.Receipt.ExitCode)
	})

	t.Run("slashing too early fails", func(t *testing.T) {
		st, vms, minerAddr := createMinerWithPower(t)

		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission, "slashStorageFault", nil)
		require.NoError(t, err)
		assert.Contains(t, res.ExecutionError.Error(), "miner not yet tardy")
		assert.Equal(t, uint8(ErrMinerNotSlashable), res.Receipt.ExitCode)

		// assert miner not slashed
		assertSlashStatus(t, st, vms, minerAddr, 2*types.OneKiBSectorSize.Uint64(), nil, types.NewIntSet())
	})

	t.Run("slashing after generation attack time succeeds", func(t *testing.T) {
		st, vms, minerAddr := createMinerWithPower(t)

		// get storage power prior to fault
		oldTotalStoragePower := th.GetTotalPower(t, st, vms)

		slashTime := lastPossibleSubmission + 1
		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, slashTime, "slashStorageFault", nil)
		require.NoError(t, err)
		require.NoError(t, res.ExecutionError)
		assert.Equal(t, uint8(0), res.Receipt.ExitCode)

		// assert miner has been slashed
		assertSlashStatus(t, st, vms, minerAddr, 0, types.NewBlockHeight(slashTime), types.NewIntSet(1, 2))

		// assert all miner power (2 small sectors worth) has been removed from totalStoragePower
		newTotalStoragePower := th.GetTotalPower(t, st, vms)
		assert.Equal(t, types.OneKiBSectorSize.Mul(types.NewBytesAmount(2)), oldTotalStoragePower.Sub(newTotalStoragePower))

		// assert proving set and sector set are also updated
		minerState := mustGetMinerState(st, vms, minerAddr)
		assert.Equal(t, 0, minerState.SectorCommitments.Size(), "slashed sectors are removed from commitments")
		assert.Equal(t, 0, minerState.ProvingSet.Size(), "slashed sectors are removed from ProvingSet")

		// assert owed collateral is set to active collateral
		// TODO: We currently do not know the correct amount of collateral: https://github.com/filecoin-project/go-filecoin/issues/3050
		assert.Equal(t, types.ZeroAttoFIL, minerState.OwedStorageCollateral)
	})

	t.Run("slashing a miner twice fails", func(t *testing.T) {
		st, vms, minerAddr := createMinerWithPower(t)

		slashTime := lastPossibleSubmission + 1
		_, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, slashTime, "slashStorageFault", nil)
		require.NoError(t, err)

		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, slashTime+1, "slashStorageFault", nil)
		require.NoError(t, err)
		assert.Contains(t, res.ExecutionError.Error(), "miner already slashed")
		assert.Equal(t, uint8(ErrMinerAlreadySlashed), res.Receipt.ExitCode)
	})
}

func assertSlashStatus(t *testing.T, st state.Tree, vms vm.StorageMap, minerAddr address.Address, power uint64,
	slashedAt *types.BlockHeight, slashed types.IntSet) {
	minerState := mustGetMinerState(st, vms, minerAddr)

	assert.Equal(t, types.NewBytesAmount(power), minerState.Power)
	assert.Equal(t, slashedAt, minerState.SlashedAt)
	assert.Equal(t, slashed, minerState.SlashedSet)

}

func TestVerifyPIP(t *testing.T) {
	tf.UnitTest(t)

	firstSectorCommitments := th.MakeCommitments()
	firstSectorCommD := firstSectorCommitments.CommD
	firstSectorID := uint64(1)
	secondSectorId := uint64(24)

	sectorSetWithOneCommitment := NewSectorSet()
	sectorSetWithOneCommitment.Add(firstSectorID, firstSectorCommitments)

	commP := th.MakeCommitment()

	t.Run("PIP is invalid if miner hasn't committed sector", func(t *testing.T) {
		vmctx, verifier, minerActor := (&minerEnvBuilder{
			message:   "verifyPieceInclusionProof",
			sectorSet: NewSectorSet(),
		}).build()

		code, err := minerActor.VerifyPieceInclusion(vmctx, th.MakeCommitment(), types.NewBytesAmount(42), secondSectorId, nil)
		require.Error(t, err)
		require.Equal(t, "sector not committed", err.Error())
		require.NotEqual(t, 0, int(code), "should not produce non-zero exit code")

		require.Nil(t, verifier.LastReceivedVerifyPieceInclusionProofRequest, "should have never called the verifier")
	})

	t.Run("PIP is invalid if miner has not submitted any PoSt proofs", func(t *testing.T) {
		vmctx, verifier, minerActor := (&minerEnvBuilder{
			message:   "verifyPieceInclusionProof",
			sectorSet: sectorSetWithOneCommitment,
			lastPoSt:  nil,
		}).build()

		code, err := minerActor.VerifyPieceInclusion(vmctx, commP, types.NewBytesAmount(66), firstSectorID, []byte{42})
		require.Error(t, err)
		assert.Equal(t, "proofs out of date", err.Error())
		assert.NotEqual(t, 0, int(code), "should not produce non-zero exit code")

		require.Nil(t, verifier.LastReceivedVerifyPieceInclusionProofRequest)
	})

	t.Run("PIP is invalid if miner's PoSt is too far into the past", func(t *testing.T) {
		vmctx, verifier, minerActor := (&minerEnvBuilder{
			message:   "verifyPieceInclusionProof",
			sectorSet: sectorSetWithOneCommitment,
			lastPoSt:  types.NewBlockHeight(0).Sub(types.NewBlockHeight(PieceInclusionGracePeriodBlocks * 2)),
		}).build()

		code, err := minerActor.VerifyPieceInclusion(vmctx, th.MakeCommitment(), types.NewBytesAmount(66), firstSectorID, []byte{42})
		require.Error(t, err)
		require.Equal(t, "proofs out of date", err.Error())
		require.NotEqual(t, 0, int(code), "should not produce non-zero exit code")

		require.Nil(t, verifier.LastReceivedVerifyPieceInclusionProofRequest, "should have never called the verifier")
	})

	t.Run("verifier errors are propagated to caller", func(t *testing.T) {
		vmctx, verifier, minerActor := (&minerEnvBuilder{
			message:   "verifyPieceInclusionProof",
			sectorSet: sectorSetWithOneCommitment,
			lastPoSt:  types.NewBlockHeight(0),
			verifier: &verification.FakeVerifier{
				VerifyPieceInclusionProofValid: false,
				VerifyPieceInclusionProofError: errors.New("wombat"),
			},
		}).build()

		code, err := minerActor.VerifyPieceInclusion(vmctx, commP, types.NewBytesAmount(66), firstSectorID, []byte{42})
		require.Error(t, err)
		require.Equal(t, "failed to verify piece inclusion proof: wombat", err.Error())
		require.NotEqual(t, 0, int(code), "should not produce non-zero exit code")

		require.NotNil(t, verifier.LastReceivedVerifyPieceInclusionProofRequest)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.CommP[:], commP)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.CommD, firstSectorCommD)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.PieceSize, types.NewBytesAmount(66))
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.PieceInclusionProof, []byte{42})
	})

	t.Run("verifier rejecting the proof produces an error, too", func(t *testing.T) {
		vmctx, verifier, minerActor := (&minerEnvBuilder{
			message:   "verifyPieceInclusionProof",
			sectorSet: sectorSetWithOneCommitment,
			lastPoSt:  types.NewBlockHeight(0),
			verifier: &verification.FakeVerifier{
				VerifyPieceInclusionProofValid: false,
			},
		}).build()

		code, err := minerActor.VerifyPieceInclusion(vmctx, commP, types.NewBytesAmount(66), firstSectorID, []byte{42})
		require.Error(t, err)
		require.Equal(t, "piece inclusion proof did not validate", err.Error())
		require.NotEqual(t, 0, int(code), "should not produce non-zero exit code")
		require.Equal(t, ErrInvalidPieceInclusionProof, int(code), "should produce invalid pip exit code")

		require.NotNil(t, verifier.LastReceivedVerifyPieceInclusionProofRequest)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.CommP[:], commP)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.CommD, firstSectorCommD)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.PieceSize, types.NewBytesAmount(66))
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.PieceInclusionProof, []byte{42})
	})

	t.Run("PIP is valid if miner's PoSts are before the end of the grace period", func(t *testing.T) {
		vmctx, verifier, minerActor := (&minerEnvBuilder{
			message:   "verifyPieceInclusionProof",
			sectorSet: sectorSetWithOneCommitment,
			lastPoSt:  types.NewBlockHeight(0),
			verifier: &verification.FakeVerifier{
				VerifyPieceInclusionProofValid: true,
			},
		}).build()

		code, err := minerActor.VerifyPieceInclusion(vmctx, commP, types.NewBytesAmount(66), firstSectorID, []byte{42})
		require.NoError(t, err)
		require.Equal(t, 0, int(code), "should be successful")

		require.NotNil(t, verifier.LastReceivedVerifyPieceInclusionProofRequest)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.CommP[:], commP)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.CommD, firstSectorCommD)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.PieceSize, types.NewBytesAmount(66))
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.PieceInclusionProof, []byte{42})
	})

	t.Run("PIP is valid if miner's PoSt is at the very end of the grace period", func(t *testing.T) {
		vmctx, verifier, minerActor := (&minerEnvBuilder{
			message:   "verifyPieceInclusionProof",
			sectorSet: sectorSetWithOneCommitment,
			lastPoSt:  types.NewBlockHeight(PieceInclusionGracePeriodBlocks),
			verifier: &verification.FakeVerifier{
				VerifyPieceInclusionProofValid: true,
			},
		}).build()

		code, err := minerActor.VerifyPieceInclusion(vmctx, commP, types.NewBytesAmount(66), firstSectorID, []byte{42})
		require.NoError(t, err)
		require.Equal(t, 0, int(code), "should be successful")

		require.NotNil(t, verifier.LastReceivedVerifyPieceInclusionProofRequest)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.CommP[:], commP)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.CommD, firstSectorCommD)
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.PieceSize, types.NewBytesAmount(66))
		require.Equal(t, verifier.LastReceivedVerifyPieceInclusionProofRequest.PieceInclusionProof, []byte{42})
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

func TestMinerGetPoStState(t *testing.T) {
	tf.UnitTest(t)

	firstCommitBlockHeight := uint64(3)

	lastHeightOfFirstPeriod := firstCommitBlockHeight + LargestSectorSizeProvingPeriodBlocks
	lastHeightOfSecondPeriod := lastHeightOfFirstPeriod + LargestSectorGenerationAttackThresholdBlocks

	faults := types.EmptyFaultSet()

	t.Run("is reported as not late within the proving period", func(t *testing.T) {
		mal := setupMinerActorLiason(t)
		mal.requireCommit(firstCommitBlockHeight, uint64(1))
		mal.requireCommit(firstCommitBlockHeight+1, uint64(2))
		mal.requireCommit(firstCommitBlockHeight+2, uint64(17))

		// submit PoSt to update proving set with no done sectors
		done := types.EmptyIntSet()
		mal.requirePoSt(firstCommitBlockHeight+5, done, faults)
		mal.assertPoStStateAtHeight(PoStStateWithinProvingPeriod, firstCommitBlockHeight)
		mal.assertPoStStateAtHeight(PoStStateWithinProvingPeriod, firstCommitBlockHeight+6)
	})

	t.Run("is reported as PoStStateAfterProvingPeriod after the proving period", func(t *testing.T) {
		mal := setupMinerActorLiason(t)
		mal.requireCommit(firstCommitBlockHeight, uint64(1))

		mal.assertPoStStateAtHeight(PoStStateAfterProvingPeriod, lastHeightOfFirstPeriod+1)
	})
	t.Run("is reported as PoStStateAfterGenerationAttackThreshold after the proving period", func(t *testing.T) {
		mal := setupMinerActorLiason(t)
		mal.requireCommit(firstCommitBlockHeight, uint64(1))

		mal.assertPoStStateAtHeight(PoStStateAfterGenerationAttackThreshold, lastHeightOfSecondPeriod)
	})

	t.Run("is reported as PoStStateNoStorage when actor has empty proving set", func(t *testing.T) {
		mal := setupMinerActorLiason(t)
		mal.assertPoStStateAtHeight(PoStStateNoStorage, firstCommitBlockHeight)
		mal.assertPoStStateAtHeight(PoStStateNoStorage, lastHeightOfFirstPeriod+1)
		mal.assertPoStStateAtHeight(PoStStateNoStorage, lastHeightOfSecondPeriod+1)
	})
}

func TestGetProvingSetCommitments(t *testing.T) {
	tf.UnitTest(t)

	message := types.NewMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, "getProvingSetCommitments", nil)
	comm1 := th.MakeCommitments()
	comm2 := th.MakeCommitments()
	comm3 := th.MakeCommitments()

	commitments := NewSectorSet()
	commitments.Add(1, types.Commitments{CommR: comm1.CommR})
	commitments.Add(2, types.Commitments{CommR: comm2.CommR})
	commitments.Add(3, types.Commitments{CommR: comm3.CommR})

	t.Run("returns only commitments that are in proving set", func(t *testing.T) {
		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.SectorCommitments = commitments

		// The 3 sector is not in the proving set, so its CommR should not appear in the VerifyPoSt request
		minerState.ProvingSet = types.NewIntSet(1, 2)

		vmctx := th.NewFakeVMContext(message, minerState)
		miner := Actor{}

		commitments, code, err := miner.GetProvingSetCommitments(vmctx)
		require.NoError(t, err)
		require.Equal(t, uint8(0), code)

		assert.Equal(t, 2, len(commitments))
		assert.Equal(t, comm1.CommR, commitments["1"].CommR)
		assert.Equal(t, comm2.CommR, commitments["2"].CommR)
	})

	t.Run("faults if proving set commitment not found in sector commitments", func(t *testing.T) {
		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.SectorCommitments = commitments

		minerState.ProvingSet = types.NewIntSet(4) // sector commitments has no sector 4

		vmctx := th.NewFakeVMContext(message, minerState)
		miner := Actor{}

		_, code, err := miner.GetProvingSetCommitments(vmctx)
		require.Error(t, err)
		assert.True(t, vmerrors.IsFault(err))
		assert.Equal(t, "proving set id, 4, missing in sector commitments", err.Error())
		assert.NotEqual(t, uint8(0), code)
	})
}

func mustDeserializeAddress(t *testing.T, result [][]byte) address.Address {
	addrValue, err := abi.Deserialize(result[0], abi.Address)
	require.NoError(t, err)

	addr, ok := addrValue.Val.(address.Address)
	require.True(t, ok)

	return addr
}

// mustGetMinerState returns the block of actor state represented by the head of the actor with the given address
func mustGetMinerState(st state.Tree, vms vm.StorageMap, a address.Address) *State {
	actor := state.MustGetActor(st, a)

	storage := vms.NewStorage(a, actor)
	data, err := storage.Get(actor.Head)
	if err != nil {
		panic(err)
	}

	minerState := &State{}
	err = cbor.DecodeInto(data, minerState)
	if err != nil {
		panic(err)
	}

	return minerState
}

type minerEnvBuilder struct {
	lastPoSt   *types.BlockHeight
	message    string
	sectorSet  SectorSet
	sectorSize *types.BytesAmount
	verifier   *verification.FakeVerifier
}

func (b *minerEnvBuilder) build() (exec.VMContext, *verification.FakeVerifier, *Actor) {
	minerState := NewState(address.TestAddress, address.TestAddress, peer.ID(""), b.sectorSize)
	minerState.SectorCommitments = b.sectorSet
	minerState.LastPoSt = b.lastPoSt

	if b.sectorSet == nil {
		b.sectorSet = NewSectorSet()
	}

	if b.sectorSize == nil {
		b.sectorSize = types.OneKiBSectorSize
	}

	if b.verifier == nil {
		b.verifier = &verification.FakeVerifier{}
	}

	vmctx := th.NewFakeVMContextWithVerifier(types.NewMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, b.message, nil), minerState, b.verifier)

	return vmctx, b.verifier, &Actor{}
}
