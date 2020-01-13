package miner_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	vmerrors "github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	internal "github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gastracker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storagemap"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestAskFunctions(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := th.RequireCreateStorages(ctx, t)

	outAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))
	minerAddr := th.RequireActorIDAddress(ctx, t, st, vms, outAddr)

	// make an ask, and then make sure it all looks good
	pdata := actor.MustConvertParams(types.NewAttoFILFromFIL(5), big.NewInt(1500))
	msg := types.NewUnsignedMessage(address.TestAddress, minerAddr, 1, types.ZeroAttoFIL, AddAsk, pdata)

	_, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(1))
	assert.NoError(t, err)

	pdata = actor.MustConvertParams(big.NewInt(0))
	msg = types.NewUnsignedMessage(address.TestAddress, minerAddr, 2, types.ZeroAttoFIL, GetAsk, pdata)
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
	msg = types.NewUnsignedMessage(address.TestAddress, minerAddr, 2, types.ZeroAttoFIL, GetAsk, pdata)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(2))
	assert.NoError(t, err)
	assert.Equal(t, Errors[ErrAskNotFound], result.ExecutionError)

	// make another ask!
	pdata = actor.MustConvertParams(types.NewAttoFILFromFIL(110), big.NewInt(200))
	msg = types.NewUnsignedMessage(address.TestAddress, minerAddr, 3, types.ZeroAttoFIL, AddAsk, pdata)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(3))
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(1), big.NewInt(0).SetBytes(result.Receipt.Return[0]))

	pdata = actor.MustConvertParams(big.NewInt(1))
	msg = types.NewUnsignedMessage(address.TestAddress, minerAddr, 4, types.ZeroAttoFIL, GetAsk, pdata)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(4))
	assert.NoError(t, err)

	var ask2 Ask
	err = actor.UnmarshalStorage(result.Receipt.Return[0], &ask2)
	require.NoError(t, err)
	assert.Equal(t, types.NewBlockHeight(203), ask2.Expiry)
	assert.Equal(t, uint64(1), ask2.ID.Uint64())

	msg = types.NewUnsignedMessage(address.TestAddress, minerAddr, 5, types.ZeroAttoFIL, GetAsks, nil)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(4))
	assert.NoError(t, err)
	assert.NoError(t, result.ExecutionError)

	var askids []types.Uint64
	require.NoError(t, actor.UnmarshalStorage(result.Receipt.Return[0], &askids))
	assert.Len(t, askids, 2)
}

func TestChangeWorker(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Change worker address", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		// retrieve worker before changing it
		result := callQueryMethodSuccess(GetWorker, ctx, t, st, vms, address.TestAddress, minerAddr)
		addr := mustDeserializeAddress(t, result)

		assert.Equal(t, address.TestAddress, addr)

		// change worker
		pdata := actor.MustConvertParams(address.TestAddress2)

		msg := types.NewUnsignedMessage(address.TestAddress, minerAddr, 1, types.ZeroAttoFIL, ChangeWorker, pdata)

		_, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(1))
		assert.NoError(t, err)

		// retrieve worker
		result = callQueryMethodSuccess(GetWorker, ctx, t, st, vms, address.TestAddress, minerAddr)
		addr = mustDeserializeAddress(t, result)

		assert.Equal(t, address.TestAddress2, addr)

		// ensure owner is not also changed
		result = callQueryMethodSuccess(GetOwner, ctx, t, st, vms, address.TestAddress, minerAddr)
		addr = mustDeserializeAddress(t, result)

		assert.Equal(t, address.TestAddress, addr)
	})

	t.Run("Only owner can change address", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		// change worker
		pdata := actor.MustConvertParams(address.TestAddress2)
		badActor := address.TestAddress2
		msg := types.NewUnsignedMessage(badActor, minerAddr, 1, types.ZeroAttoFIL, ChangeWorker, pdata)

		result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(1))
		assert.NoError(t, err)
		require.Error(t, result.ExecutionError)
		assert.Contains(t, result.ExecutionError.Error(), "not authorized")
		assert.Equal(t, uint8(ErrCallerUnauthorized), result.Receipt.ExitCode)
	})

	t.Run("Errors when gas cost too low", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)
		outAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))
		minerAddr := th.RequireActorIDAddress(ctx, t, st, vms, outAddr)
		mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

		// change worker
		pdata := actor.MustConvertParams(address.TestAddress2)
		gasPrice, _ := types.NewAttoFILFromFILString(".00001")
		gasLimit := types.NewGasUnits(10)
		msg := types.NewMeteredMessage(mockSigner.Addresses[0], minerAddr, 0, types.ZeroAttoFIL, ChangeWorker, pdata, gasPrice, gasLimit)

		result, err := th.ApplyTestMessageWithGas(builtin.DefaultActors, st, vms, msg, types.NewBlockHeight(1), mockSigner.Addresses[0])
		assert.NoError(t, err)

		require.Error(t, result.ExecutionError)
		assert.Contains(t, result.ExecutionError.Error(), "Insufficient gas")
		assert.Equal(t, uint8(internal.ErrInsufficientGas), result.Receipt.ExitCode)
	})
}

func TestConstructor(t *testing.T) {
	tf.UnitTest(t)

	message := types.NewUnsignedMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, types.ConstructorMethodID, nil)
	vmctx := vm.NewFakeVMContext(message, nil)
	storageMap := th.VMStorage()
	minerActor := actor.NewActor(types.MinerActorCodeCid, types.ZeroAttoFIL)
	vmctx.LegacyStorageValue = storageMap.NewStorage(address.TestAddress, minerActor)

	act := &Actor{}
	addrGetter := address.NewForTestGetter()
	owner := addrGetter()
	worker := addrGetter()
	sectorSize := types.NewBytesAmount(42)
	pid := th.RequireIntPeerID(t, 3)

	(*Impl)(act).Constructor(vmctx, owner, worker, pid, sectorSize)

	stateEncoded, err := vmctx.LegacyStorage().Get(minerActor.Head)
	require.NoError(t, err)

	state := &State{}
	err = encoding.Decode(stateEncoded, state)
	require.NoError(t, err)

	assert.Equal(t, state.Owner, owner)
	assert.Equal(t, state.Worker, worker)
	assert.Equal(t, state.PeerID, pid)
	assert.Equal(t, state.SectorSize, sectorSize)
}

func TestGetWorker(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := th.RequireCreateStorages(ctx, t)

	minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

	// retrieve worker
	result := callQueryMethodSuccess(GetWorker, ctx, t, st, vms, address.TestAddress, minerAddr)

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
	result := callQueryMethodSuccess(GetOwner, ctx, t, st, vms, address.TestAddress, minerAddr)

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
	result := callQueryMethodSuccess(GetActiveCollateral, ctx, t, st, vms, address.TestAddress, minerAddr)
	attoFILValue, err := abi.Deserialize(result[0], abi.AttoFIL)
	require.NoError(t, err)

	coll, ok := attoFILValue.Val.(types.AttoFIL)
	require.True(t, ok)

	assert.Equal(t, types.ZeroAttoFIL, coll)

	// Commit a sector.
	commR := th.MakeCommitment()
	commRStar := th.MakeCommitment()
	commD := th.MakeCommitment()

	res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, CommitSector, nil, uint64(0), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)

	// Check updated collateral
	result = callQueryMethodSuccess(GetActiveCollateral, ctx, t, st, vms, address.TestAddress, minerAddr)
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
		resultA := callQueryMethodSuccess(GetPeerID, ctx, t, st, vms, address.TestAddress, minerAddr)
		pid, err := peer.IDFromBytes(resultA[0])
		require.NoError(t, err)

		require.Equal(t, peer.IDB58Encode(origPid), peer.IDB58Encode(pid))

		// update peer ID
		newPid := th.RequireRandomPeerID(t)
		updatePeerIdSuccess(t, st, vms, address.TestAddress, minerAddr, newPid)

		// retrieve peer ID
		resultB := callQueryMethodSuccess(GetPeerID, ctx, t, st, vms, address.TestAddress, minerAddr)
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
		updatePeerIdMsg := types.NewUnsignedMessage(
			address.TestAddress2,
			minerAddr,
			th.RequireGetNonce(t, st, vms, address.TestAddress2),
			types.NewAttoFILFromFIL(0),
			UpdatePeerID,
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
		result := callQueryMethodSuccess(GetPower, ctx, t, st, vms, address.TestAddress, minerAddr)
		require.True(t, types.ZeroBytes.Equal(types.NewBytesAmountFromBytes(result[0])))
	})
}

func TestMinerGetProvingPeriod(t *testing.T) {
	tf.UnitTest(t)

	t.Run("GetProvingWindow returns unitialized values when proving period is unset", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := th.RequireCreateStorages(ctx, t)

		minerAddr := th.CreateTestMinerWith(types.NewAttoFILFromFIL(240), t, st, vms, address.TestAddress, th.RequireRandomPeerID(t), 0)

		// retrieve proving period
		result := callQueryMethodSuccess(GetProvingWindow, ctx, t, st, vms, address.TestAddress, minerAddr)
		window, err := abi.Deserialize(result[0], abi.UintArray)
		require.NoError(t, err)
		start := window.Val.([]types.Uint64)[0]
		end := window.Val.([]types.Uint64)[1]

		assert.Equal(t, types.Uint64(0), start)
		assert.Equal(t, types.Uint64(0), end)
	})

	t.Run("GetProvingWindow returns the start and end of the proving period", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		st, vms := th.RequireCreateStorages(ctx, t)

		minerAddr := th.CreateTestMinerWith(types.NewAttoFILFromFIL(240), t, st, vms, address.TestAddress, th.RequireRandomPeerID(t), 0)

		// commit sector to set ProvingPeriodEnd
		commR := th.MakeCommitment()
		commRStar := th.MakeCommitment()
		commD := th.MakeCommitment()

		blockHeight := uint64(42)
		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, blockHeight, CommitSector, nil, uint64(1), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
		require.NoError(t, err)
		require.NoError(t, res.ExecutionError)
		require.Equal(t, uint8(0), res.Receipt.ExitCode)

		// retrieve proving period
		result := callQueryMethodSuccess(GetProvingWindow, ctx, t, st, vms, address.TestAddress, minerAddr)
		window, err := abi.Deserialize(result[0], abi.UintArray)
		require.NoError(t, err)
		start := window.Val.([]types.Uint64)[0]
		end := window.Val.([]types.Uint64)[1]

		// end of proving period is now plus proving period size
		expectedEnd := blockHeight + uint64(LargestSectorSizeProvingPeriodBlocks)
		// start of proving period is end minus the PoSt challenge time
		expectedStart := expectedEnd - PoStChallengeWindowBlocks

		assert.Equal(t, types.Uint64(expectedStart), start)
		assert.Equal(t, types.Uint64(expectedEnd), end)
	})
}

func updatePeerIdSuccess(t *testing.T, st state.Tree, vms storagemap.StorageMap, fromAddr address.Address, minerAddr address.Address, newPid peer.ID) {
	updatePeerIdMsg := types.NewUnsignedMessage(
		fromAddr,
		minerAddr,
		th.RequireGetNonce(t, st, vms, fromAddr),
		types.NewAttoFILFromFIL(0),
		UpdatePeerID,
		actor.MustConvertParams(newPid))

	applyMsgResult, err := th.ApplyTestMessage(st, vms, updatePeerIdMsg, types.NewBlockHeight(0))
	require.NoError(t, err)
	require.NoError(t, applyMsgResult.ExecutionError)
	require.Equal(t, uint8(0), applyMsgResult.Receipt.ExitCode)
}

func callQueryMethodSuccess(method types.MethodID,
	ctx context.Context,
	t *testing.T, st state.Tree,
	vms storagemap.StorageMap,
	fromAddr address.Address,
	minerAddr address.Address) [][]byte {
	res, code, err := consensus.NewDefaultProcessor().CallQueryMethod(ctx, st, vms, minerAddr, method, []byte{}, fromAddr, nil)
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
			return th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, CommitSector, nil, uint64(sectorId), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
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

		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, CommitSector, nil, uint64(1), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
		require.NoError(t, err)
		require.NoError(t, res.ExecutionError)
		require.Equal(t, uint8(0), res.Receipt.ExitCode)

		// check that the proving period matches
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, GetProvingWindow, nil)
		require.NoError(t, err)
		require.NoError(t, res.ExecutionError)

		// provingPeriodEnd is block height plus proving period
		provingPeriod := ProvingPeriodDuration(types.OneKiBSectorSize)

		// blockheight was 3
		window, err := abi.Deserialize(res.Receipt.Return[0], abi.UintArray)
		require.NoError(t, err)
		require.Equal(t, types.NewBlockHeight(3+provingPeriod), types.NewBlockHeight(uint64(window.Val.([]types.Uint64)[1])))

		// fail because commR already exists
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 4, CommitSector, nil, uint64(1), commD, commR, commRStar, th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
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
	vms           storagemap.StorageMap
	ancestors     []block.TipSet
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
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, mal.minerAddr, 0, blockHeight, CommitSector, mal.ancestors, sectorID, th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(mal.t, err)
	require.NoError(mal.t, res.ExecutionError)
	require.Equal(mal.t, uint8(0), res.Receipt.ExitCode)
}

func (mal *minerActorLiason) requirePoSt(blockHeight uint64, done types.IntSet, faults types.FaultSet) {
	mal.requireHeightNotPast(blockHeight)
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, mal.minerAddr, 0, blockHeight, SubmitPoSt, mal.ancestors, th.MakeRandomPoStProofForTest(), faults, done)
	assert.NoError(mal.t, err)
	assert.NoError(mal.t, res.ExecutionError)
	assert.Equal(mal.t, uint8(0), res.Receipt.ExitCode)
}

func (mal *minerActorLiason) requireReadState() State {
	miner := state.MustGetActor(mal.st, mal.minerAddr)
	storage := mal.vms.NewStorage(mal.minerAddr, miner)
	stateBytes, err := storage.Get(storage.LegacyHead())
	require.NoError(mal.t, err)
	var minerState State
	err = actor.UnmarshalStorage(stateBytes, &minerState)
	require.NoError(mal.t, err)
	return minerState
}

func (mal *minerActorLiason) requirePower(queryHeight uint64) *types.BytesAmount {
	mal.requireHeightNotPast(queryHeight)
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, mal.minerAddr, 0, queryHeight, GetPower, mal.ancestors)
	require.NoError(mal.t, err)
	require.NoError(mal.t, res.ExecutionError)
	require.Equal(mal.t, uint8(0), res.Receipt.ExitCode)
	require.Equal(mal.t, 1, len(res.Receipt.Return))
	return types.NewBytesAmountFromBytes(res.Receipt.Return[0])
}

func (mal *minerActorLiason) requireTotalStorage(queryHeight uint64) *types.BytesAmount {
	mal.requireHeightNotPast(queryHeight)
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, address.StorageMarketAddress, 0, queryHeight, storagemarket.GetTotalStorage, mal.ancestors)
	require.NoError(mal.t, err)
	require.NoError(mal.t, res.ExecutionError)
	require.Equal(mal.t, uint8(0), res.Receipt.ExitCode)
	require.Equal(mal.t, 1, len(res.Receipt.Return))
	return types.NewBytesAmountFromBytes(res.Receipt.Return[0])
}

func (mal *minerActorLiason) assertPoStFail(blockHeight uint64, done types.IntSet, exitCode uint8) {
	mal.requireHeightNotPast(blockHeight)
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, mal.minerAddr, 0, blockHeight, SubmitPoSt, mal.ancestors, th.MakeRandomPoStProofForTest(), types.EmptyFaultSet(), done)
	assert.NoError(mal.t, err)
	assert.Error(mal.t, res.ExecutionError)
	assert.Equal(mal.t, exitCode, res.Receipt.ExitCode)
}

func (mal *minerActorLiason) assertPoStStateAtHeight(expected int64, queryHeight uint64) {
	res, err := th.CreateAndApplyTestMessage(mal.t, mal.st, mal.vms, mal.minerAddr, 0, queryHeight, GetPoStState, mal.ancestors)
	assert.NoError(mal.t, err)
	require.NotNil(mal.t, res)

	ret, err := abi.Deserialize(res.Receipt.Return[0], abi.Integer)
	require.NoError(mal.t, err)

	assert.Equal(mal.t, big.NewInt(expected), ret.Val)
}

func newMinerActorLiason(t *testing.T, st state.Tree, vms storagemap.StorageMap, ancestors []block.TipSet, minerAddr address.Address) *minerActorLiason {
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
	head := builder.AppendManyOn(10, block.UndefTipSet)
	ancestors := builder.RequireTipSets(head.Key(), 10)
	origPid := th.RequireRandomPeerID(t)
	outAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, origPid)
	minerAddr := th.RequireActorIDAddress(ctx, t, st, vms, outAddr)
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

	message := types.NewUnsignedMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, SubmitPoSt, nil)
	comm1 := th.MakeCommitments()
	comm2 := th.MakeCommitments()
	comm3 := th.MakeCommitments()

	t.Run("Sends correct parameters to post verifier", func(t *testing.T) {
		t.Skip("Skip pending election post with new sectorbuilder")
		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(ProvingPeriodDuration(types.OneKiBSectorSize))
		minerState.SectorCommitments = NewSectorSet()
		minerState.SectorCommitments.Add(1, types.Commitments{CommR: comm1.CommR})
		minerState.SectorCommitments.Add(2, types.Commitments{CommR: comm2.CommR})
		minerState.SectorCommitments.Add(3, types.Commitments{CommR: comm3.CommR})

		// The 3 sector is not in the proving set, so its CommR should not appear in the VerifyFallbackPoSt request
		minerState.ProvingSet = types.NewIntSet(1, 2)
		verifier := &verification.FakeVerifier{
			VerifyPoStValid: true,
		}
		vmctx := vm.NewFakeVMContextWithVerifier(message, minerState, verifier)
		vmctx.BlockHeightValue = types.NewBlockHeight(530)

		miner := Impl(Actor{Bootstrap: false})

		testProof := th.MakeRandomPoStProofForTest()
		_, err := miner.SubmitPoSt(vmctx, testProof, types.EmptyFaultSet(), types.EmptyIntSet())
		require.NoError(t, err)

		require.NotNil(t, verifier.LastReceivedVerifyPoStRequest)
		assert.Equal(t, types.OneKiBSectorSize, verifier.LastReceivedVerifyPoStRequest.SectorSize)

		seed := types.PoStChallengeSeed{}
		copy(seed[:], vmctx.RandomnessValue)

		sortedRs := ffi.NewSortedPublicSectorInfo(
			ffi.PublicSectorInfo{CommR: comm1.CommR},
			ffi.PublicSectorInfo{CommR: comm2.CommR},
		)

		assert.Equal(t, testProof, verifier.LastReceivedVerifyPoStRequest.Proof)
		assert.Equal(t, 2, len(verifier.LastReceivedVerifyPoStRequest.SectorInfo.Values()))
		assert.Equal(t, sortedRs.Values()[0].CommR, verifier.LastReceivedVerifyPoStRequest.SectorInfo.Values()[0].CommR)
		assert.Equal(t, sortedRs.Values()[1].CommR, verifier.LastReceivedVerifyPoStRequest.SectorInfo.Values()[1].CommR)
	})

	t.Run("Faults if proving set commitment is missing from sector commitments", func(t *testing.T) {
		t.Skip("Skip pending election post with new sectorbuilder")
		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(ProvingPeriodDuration(types.OneKiBSectorSize))
		minerState.SectorCommitments = NewSectorSet()

		minerState.ProvingSet = types.NewIntSet(4)
		vmctx := vm.NewFakeVMContext(message, minerState)
		vmctx.BlockHeightValue = types.NewBlockHeight(530)

		miner := Impl(Actor{Bootstrap: false})

		testProof := th.MakeRandomPoStProofForTest()
		code, err := miner.SubmitPoSt(vmctx, testProof, types.EmptyFaultSet(), types.NewIntSet())
		require.Error(t, err)
		assert.Equal(t, "miner ProvingSet sector id 4 missing in SectorCommitments", err.Error())
		assert.True(t, vmerrors.IsFault(err))
		assert.Equal(t, uint8(1), code)
	})

	t.Run("Reverts if verification errors", func(t *testing.T) {
		t.Skip("Skip pending election post with new sectorbuilder")
		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(ProvingPeriodDuration(types.OneKiBSectorSize))
		minerState.SectorCommitments = NewSectorSet()
		minerState.SectorCommitments.Add(1, types.Commitments{CommR: comm1.CommR})

		minerState.ProvingSet = types.NewIntSet(1)
		verifier := &verification.FakeVerifier{
			VerifyPoStError: errors.New("verifier error"),
		}

		vmctx := vm.NewFakeVMContextWithVerifier(message, minerState, verifier)
		vmctx.BlockHeightValue = types.NewBlockHeight(530)

		miner := Impl(Actor{Bootstrap: false})

		testProof := th.MakeRandomPoStProofForTest()
		code, err := miner.SubmitPoSt(vmctx, testProof, types.EmptyFaultSet(), types.NewIntSet())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "verifier error")
		assert.True(t, vmerrors.ShouldRevert(err))
		assert.Equal(t, uint8(1), code)
	})

	t.Run("Reverts if proof is invalid", func(t *testing.T) {
		t.Skip("Skip pending election post with new sectorbuilder")
		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(ProvingPeriodDuration(types.OneKiBSectorSize))
		minerState.SectorCommitments = NewSectorSet()
		minerState.SectorCommitments.Add(1, types.Commitments{CommR: comm1.CommR})

		minerState.ProvingSet = types.NewIntSet(1)
		verifier := &verification.FakeVerifier{
			VerifyPoStValid: false,
		}

		vmctx := vm.NewFakeVMContextWithVerifier(message, minerState, verifier)
		vmctx.BlockHeightValue = types.NewBlockHeight(530)

		miner := Impl(Actor{Bootstrap: false})

		testProof := th.MakeRandomPoStProofForTest()
		code, err := miner.SubmitPoSt(vmctx, testProof, types.EmptyFaultSet(), types.NewIntSet())
		require.Error(t, err)
		assert.Equal(t, Errors[ErrInvalidPoSt], err)
		assert.True(t, vmerrors.ShouldRevert(err))
		assert.Equal(t, uint8(ErrInvalidPoSt), code)
	})
}

func TestMinerSubmitPoStProvingSet(t *testing.T) {
	t.Skip("Skip pending election post with new sectorbuilder")
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
	t.Skip("Skip pending election post with new sectorbuilder")
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
	t.Skip("Skip pending election post with new sectorbuilder")
	tf.UnitTest(t)

	ctx := context.Background()
	st, vms := th.RequireCreateStorages(ctx, t)

	builder := chain.NewBuilder(t, address.Undef)
	head := builder.AppendManyOn(10, block.UndefTipSet)
	ancestors := builder.RequireTipSets(head.Key(), 10)
	origPid := th.RequireRandomPeerID(t)
	outAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, origPid)
	minerAddr := th.RequireActorIDAddress(ctx, t, st, vms, outAddr)
	proof := th.MakeRandomPoStProofForTest()
	doneDefault := types.EmptyIntSet()
	faultsDefault := types.EmptyFaultSet()

	miner := state.MustGetActor(st, minerAddr)
	minerBalance := miner.Balance
	owner, _ := th.RequireLookupActor(ctx, t, st, vms, address.TestAddress)
	ownerBalance := owner.Balance

	firstCommitBlockHeight := uint64(3)
	secondProvingPeriodStart := LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight
	secondProvingPeriodEnd := 2*LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight
	lastPossibleSubmission := secondProvingPeriodStart + 2*LargestSectorSizeProvingPeriodBlocks - 1

	// add a sector
	res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight, CommitSector, ancestors, uint64(1), th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)

	// add another sector
	res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight+1, CommitSector, ancestors, uint64(2), th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)

	t.Run("on-time PoSt succeeds", func(t *testing.T) {
		// submit post
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight+5, SubmitPoSt, ancestors, proof, faultsDefault, doneDefault)
		assert.NoError(t, err)
		assert.NoError(t, res.ExecutionError)
		assert.Equal(t, uint8(0), res.Receipt.ExitCode)

		// check that the proving period is now the next one
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight+6, GetProvingWindow, ancestors)
		assert.NoError(t, err)
		assert.NoError(t, res.ExecutionError)

		window, err := abi.Deserialize(res.Receipt.Return[0], abi.UintArray)
		require.NoError(t, err)
		end := window.Val.([]types.Uint64)[1]

		assert.Equal(t, uint64(end), secondProvingPeriodEnd)
	})

	t.Run("after proving period grace period PoSt is rejected", func(t *testing.T) {
		// Rejected one block late
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission+1, SubmitPoSt, ancestors, proof, faultsDefault, doneDefault)
		assert.NoError(t, err)
		assert.Error(t, res.ExecutionError)
	})

	t.Run("late submission charged fee", func(t *testing.T) {
		// Rejected on the deadline with message value not carrying sufficient fees
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission, SubmitPoSt, ancestors, proof, faultsDefault, doneDefault)
		assert.NoError(t, err)
		assert.Error(t, res.ExecutionError)

		// Accepted on the deadline with a fee
		// Must calculate fee before submitting the PoSt, since submission will reset the proving period.
		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission, CalculateLateFee, ancestors, lastPossibleSubmission)
		fee := types.NewAttoFILFromBytes(res.Receipt.Return[0])
		require.False(t, fee.IsZero())

		res, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 1, lastPossibleSubmission, SubmitPoSt, ancestors, proof, faultsDefault, doneDefault)
		assert.NoError(t, err)
		assert.NoError(t, res.ExecutionError)
		assert.Equal(t, uint8(0), res.Receipt.ExitCode)

		// Check miner's balance unchanged (because it's topped up from message value then fee burnt).
		miner := state.MustGetActor(st, minerAddr)
		assert.Equal(t, minerBalance.String(), miner.Balance.String())

		// Check  change was refunded to owner, balance is now reduced by fee.
		owner, _ := th.RequireLookupActor(ctx, t, st, vms, address.TestAddress)
		assert.Equal(t, ownerBalance.Sub(fee).String(), owner.Balance.String())
	})

	t.Run("computes seed randomness at correct chain height when post is on time", func(t *testing.T) {
		var actualSampleHeight *types.BlockHeight

		message := types.NewUnsignedMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, SubmitPoSt, []byte{})

		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(secondProvingPeriodEnd)

		vmctx := vm.NewFakeVMContext(message, minerState)
		vmctx.VerifierValue = &verification.FakeVerifier{VerifyPoStValid: true}

		vmctx.Sampler = func(sampleHeight *types.BlockHeight) ([]byte, error) {
			actualSampleHeight = sampleHeight
			return []byte{42}, nil
		}

		// block time that isn't late
		vmctx.BlockHeightValue = types.NewBlockHeight(secondProvingPeriodEnd - PoStChallengeWindowBlocks + 30)

		miner := Impl(Actor{})
		code, err := miner.SubmitPoSt(vmctx, []byte{}, types.EmptyFaultSet(), types.EmptyIntSet())
		require.NoError(t, err)
		require.Equal(t, uint8(0), code)

		// expect to sample randomness at beginning of proving period window before proving period end
		expectedSampleHeight := types.NewBlockHeight(secondProvingPeriodEnd - PoStChallengeWindowBlocks)
		assert.Equal(t, expectedSampleHeight, actualSampleHeight)
	})

	t.Run("computes seed randomness at correct chain height when post is late", func(t *testing.T) {
		var actualSampleHeight *types.BlockHeight

		message := types.NewUnsignedMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, SubmitPoSt, []byte{})

		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(secondProvingPeriodEnd)

		vmctx := vm.NewFakeVMContext(message, minerState)
		vmctx.VerifierValue = &verification.FakeVerifier{VerifyPoStValid: true}

		vmctx.Sampler = func(sampleHeight *types.BlockHeight) ([]byte, error) {
			actualSampleHeight = sampleHeight
			return []byte{42}, nil
		}

		// block time that is late
		vmctx.BlockHeightValue = types.NewBlockHeight(secondProvingPeriodEnd + LargestSectorSizeProvingPeriodBlocks - PoStChallengeWindowBlocks + 30)

		miner := Impl(Actor{})
		code, err := miner.SubmitPoSt(vmctx, []byte{}, types.EmptyFaultSet(), types.EmptyIntSet())
		require.NoError(t, err)
		require.Equal(t, uint8(0), code)

		// expect to sample randomness at beginning of proving period window after proving period end
		expectedSampleHeight := types.NewBlockHeight(secondProvingPeriodEnd + LargestSectorSizeProvingPeriodBlocks - PoStChallengeWindowBlocks)
		assert.Equal(t, expectedSampleHeight, actualSampleHeight)
	})

	t.Run("provides informative error when PoSt attempts to sample chain height before it is ready", func(t *testing.T) {
		message := types.NewUnsignedMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, SubmitPoSt, []byte{})

		minerState := *NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(secondProvingPeriodEnd)

		vmctx := vm.NewFakeVMContext(message, minerState)
		vmctx.VerifierValue = &verification.FakeVerifier{VerifyPoStValid: true}

		vmctx.Sampler = func(sampleHeight *types.BlockHeight) ([]byte, error) {
			return []byte{}, errors.New("chain randomness unavailable")
		}

		// block time before proving window
		vmctx.BlockHeightValue = types.NewBlockHeight(secondProvingPeriodEnd + LargestSectorSizeProvingPeriodBlocks - PoStChallengeWindowBlocks - 25)

		miner := Impl(Actor{})
		code, err := miner.SubmitPoSt(vmctx, []byte{}, types.EmptyFaultSet(), types.EmptyIntSet())
		require.Error(t, err)
		require.NotEqual(t, uint8(0), code)

		assert.Contains(t, err.Error(), fmt.Sprintf("PoSt arrived at %s, which is before proving window (%d-%d)",
			vmctx.BlockHeightValue.String(),
			secondProvingPeriodEnd+LargestSectorSizeProvingPeriodBlocks-PoStChallengeWindowBlocks,
			secondProvingPeriodEnd+LargestSectorSizeProvingPeriodBlocks))
	})

}

func TestAddFaults(t *testing.T) {
	tf.UnitTest(t)

	firstCommitBlockHeight := uint64(3)
	provingPeriodStart := LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight
	provingPeriodEnd := provingPeriodStart + LargestSectorSizeProvingPeriodBlocks
	provingWindowStart := provingPeriodEnd - PoStChallengeWindowBlocks

	message := types.NewUnsignedMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, AddFaults, []byte{})

	cases := []struct {
		bh              uint64
		faults          []uint64
		initialCurrent  []uint64
		initialNext     []uint64
		expectedCurrent []uint64
		expectedNext    []uint64
	}{
		// Adding faults before the proving window adds to the CurrentFaultSet
		{provingWindowStart - 1, []uint64{42}, []uint64{1}, []uint64{}, []uint64{1, 42}, []uint64{}},

		// Adding faults after the proving window has started adds to NextFaultSet
		{provingWindowStart + 1, []uint64{42}, []uint64{}, []uint64{1}, []uint64{}, []uint64{1, 42}},
	}

	for _, tc := range cases {
		minerState := NewState(address.TestAddress, address.TestAddress, peer.ID(""), types.OneKiBSectorSize)
		minerState.ProvingPeriodEnd = types.NewBlockHeight(provingPeriodEnd)
		minerState.CurrentFaultSet = types.NewIntSet(tc.initialCurrent...)
		minerState.NextFaultSet = types.NewIntSet(tc.initialNext...)

		vmctx := vm.NewFakeVMContext(message, minerState)
		vmctx.BlockHeightValue = types.NewBlockHeight(tc.bh)

		miner := Impl(Actor{})
		code, err := miner.AddFaults(vmctx, types.NewFaultSet(tc.faults))
		require.NoError(t, err)
		require.Equal(t, uint8(0), code)

		err = actor.ReadState(vmctx, &minerState)
		require.NoError(t, err)

		require.Equal(t, len(tc.expectedCurrent), minerState.CurrentFaultSet.Size())
		require.True(t, minerState.CurrentFaultSet.HasSubset(types.NewIntSet(tc.expectedCurrent...)))

		require.Equal(t, len(tc.expectedNext), minerState.NextFaultSet.Size())
		require.True(t, minerState.NextFaultSet.HasSubset(types.NewIntSet(tc.expectedNext...)))
	}
}

func TestActorSlashStorageFault(t *testing.T) {
	tf.UnitTest(t)

	firstCommitBlockHeight := uint64(3)
	secondProvingPeriodStart := firstCommitBlockHeight + ProvingPeriodDuration(types.OneKiBSectorSize)
	thirdProvingPeriodStart := secondProvingPeriodStart + ProvingPeriodDuration(types.OneKiBSectorSize)
	thirdProvingPeriodEnd := thirdProvingPeriodStart + ProvingPeriodDuration(types.OneKiBSectorSize)
	lastPossibleSubmission := thirdProvingPeriodEnd + LargestSectorSizeProvingPeriodBlocks

	// CreateTestMiner creates a new test miner with the given peerID and miner
	// owner address and a given number of committed sectors
	createMinerWithPower := func(t *testing.T) (state.Tree, storagemap.StorageMap, address.Address) {
		ctx := context.Background()
		st, vms := th.RequireCreateStorages(ctx, t)
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		builder := chain.NewBuilder(t, address.Undef)
		head := builder.AppendManyOn(10, block.UndefTipSet)
		ancestors := builder.RequireTipSets(head.Key(), 10)
		proof := th.MakeRandomPoStProofForTest()
		doneDefault := types.EmptyIntSet()
		faultsDefault := types.EmptyFaultSet()

		// add a sector
		_, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight, CommitSector, ancestors, uint64(1), th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
		require.NoError(t, err)

		// add another sector (not in proving set yet)
		_, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, firstCommitBlockHeight+1, CommitSector, ancestors, uint64(2), th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
		require.NoError(t, err)

		// submit post (first sector only)
		_, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, secondProvingPeriodStart, SubmitPoSt, ancestors, proof, faultsDefault, doneDefault)
		require.NoError(t, err)

		// submit post (both sectors
		_, err = th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, thirdProvingPeriodStart, SubmitPoSt, ancestors, proof, faultsDefault, doneDefault)
		assert.NoError(t, err)

		return st, vms, minerAddr
	}

	t.Run("slashing charges gas", func(t *testing.T) {
		st, vms, outAddr := createMinerWithPower(t)
		minerAddr := th.RequireActorIDAddress(context.TODO(), t, st, vms, outAddr)
		mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

		// change worker
		gasPrice, _ := types.NewAttoFILFromFILString(".00001")
		gasLimit := types.NewGasUnits(10)
		msg := types.NewMeteredMessage(mockSigner.Addresses[0], minerAddr, 0, types.ZeroAttoFIL, SlashStorageFault, []byte{}, gasPrice, gasLimit)

		result, err := th.ApplyTestMessageWithGas(builtin.DefaultActors, st, vms, msg, types.NewBlockHeight(1), mockSigner.Addresses[0])
		require.NoError(t, err)

		require.Error(t, result.ExecutionError)
		assert.Contains(t, result.ExecutionError.Error(), "Insufficient gas")
		assert.Equal(t, uint8(internal.ErrInsufficientGas), result.Receipt.ExitCode)
	})

	t.Run("slashing a miner with no storage fails", func(t *testing.T) {
		ctx := context.Background()
		st, vms := th.RequireCreateStorages(ctx, t)
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission+1, SlashStorageFault, nil)
		require.NoError(t, err)
		assert.Contains(t, res.ExecutionError.Error(), "miner is inactive")
		assert.Equal(t, uint8(ErrMinerNotSlashable), res.Receipt.ExitCode)
	})

	t.Run("slashing too early fails", func(t *testing.T) {
		st, vms, outAddr := createMinerWithPower(t)
		minerAddr := th.RequireActorIDAddress(context.TODO(), t, st, vms, outAddr)

		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, lastPossibleSubmission, SlashStorageFault, nil)
		require.NoError(t, err)
		assert.Contains(t, res.ExecutionError.Error(), "miner not yet tardy")
		assert.Equal(t, uint8(ErrMinerNotSlashable), res.Receipt.ExitCode)

		// assert miner not slashed
		assertSlashStatus(t, st, vms, minerAddr, 2*types.OneKiBSectorSize.Uint64(), types.NewBlockHeight(0), types.NewIntSet())
	})

	t.Run("slashing after generation attack time succeeds", func(t *testing.T) {
		st, vms, outAddr := createMinerWithPower(t)
		minerAddr := th.RequireActorIDAddress(context.TODO(), t, st, vms, outAddr)

		// get storage power prior to fault
		oldTotalStoragePower := th.GetTotalPower(t, st, vms)

		slashTime := lastPossibleSubmission + 1
		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, slashTime, SlashStorageFault, nil)
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
		_, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, slashTime, SlashStorageFault, nil)
		require.NoError(t, err)

		res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, slashTime+1, SlashStorageFault, nil)
		require.NoError(t, err)
		assert.Contains(t, res.ExecutionError.Error(), "miner already slashed")
		assert.Equal(t, uint8(ErrMinerAlreadySlashed), res.Receipt.ExitCode)
	})
}

func assertSlashStatus(t *testing.T, st state.Tree, vms storagemap.StorageMap, minerAddr address.Address, power uint64,
	slashedAt *types.BlockHeight, slashed types.IntSet) {
	minerState := mustGetMinerState(st, vms, minerAddr)

	assert.Equal(t, types.NewBytesAmount(power), minerState.Power)
	assert.Equal(t, slashedAt, minerState.SlashedAt)
	assert.Equal(t, slashed, minerState.SlashedSet)

}

func TestGetProofsMode(t *testing.T) {
	ctx := context.Background()

	cst := hamt.NewCborStore()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)

	gasTracker := gastracker.NewLegacyGasTracker()
	gasTracker.MsgGasLimit = 99999

	t.Run("in TestMode", func(t *testing.T) {
		st := state.NewTree(cst)
		vms := vm.NewStorageMap(bs)
		vmCtx := vmcontext.NewVMContext(vmcontext.NewContextParams{
			From:        &actor.Actor{},
			To:          &actor.Actor{},
			Message:     &types.UnsignedMessage{},
			State:       state.NewCachedTree(st),
			StorageMap:  vms,
			GasTracker:  gasTracker,
			BlockHeight: types.NewBlockHeight(0),
			Ancestors:   []block.TipSet{},
			Actors:      builtin.DefaultActors,
		})

		require.NoError(t, consensus.SetupDefaultActors(ctx, st, vms, types.TestProofsMode, "test"))

		mode, err := GetProofsMode(vmCtx)
		require.NoError(t, err)
		assert.Equal(t, types.TestProofsMode, mode)
	})

	t.Run("in LiveMode", func(t *testing.T) {
		st := state.NewTree(cst)
		vms := vm.NewStorageMap(bs)
		vmCtx := vmcontext.NewVMContext(vmcontext.NewContextParams{
			From:        &actor.Actor{},
			To:          &actor.Actor{},
			Message:     &types.UnsignedMessage{},
			State:       state.NewCachedTree(st),
			StorageMap:  vms,
			GasTracker:  gasTracker,
			BlockHeight: types.NewBlockHeight(0),
			Ancestors:   []block.TipSet{},
			Actors:      builtin.DefaultActors,
		})

		require.NoError(t, consensus.SetupDefaultActors(ctx, st, vms, types.LiveProofsMode, "main"))

		mode, err := GetProofsMode(vmCtx)
		require.NoError(t, err)
		assert.Equal(t, types.LiveProofsMode, mode)
	})
}

func TestMinerGetPoStState(t *testing.T) {
	tf.UnitTest(t)

	firstCommitBlockHeight := uint64(3)

	lastHeightOfFirstPeriod := firstCommitBlockHeight + LargestSectorSizeProvingPeriodBlocks
	lastHeightOfSecondPeriod := lastHeightOfFirstPeriod + LargestSectorSizeProvingPeriodBlocks

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
	t.Run("is reported as PoStStateUnrecoverable after the proving period", func(t *testing.T) {
		mal := setupMinerActorLiason(t)
		mal.requireCommit(firstCommitBlockHeight, uint64(1))

		mal.assertPoStStateAtHeight(PoStStateUnrecoverable, lastHeightOfSecondPeriod)
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

	message := types.NewUnsignedMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, GetProvingSetCommitments, nil)
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

		// The 3 sector is not in the proving set, so its CommR should not appear in the VerifyFallbackPoSt request
		minerState.ProvingSet = types.NewIntSet(1, 2)

		vmctx := vm.NewFakeVMContext(message, minerState)
		miner := Impl(Actor{})

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

		vmctx := vm.NewFakeVMContext(message, minerState)
		miner := Impl(Actor{})

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
func mustGetMinerState(st state.Tree, vms storagemap.StorageMap, a address.Address) *State {
	actor := state.MustGetActor(st, a)

	storage := vms.NewStorage(a, actor)
	data, err := storage.Get(actor.Head)
	if err != nil {
		panic(err)
	}

	minerState := &State{}
	err = encoding.Decode(data, minerState)
	if err != nil {
		panic(err)
	}

	return minerState
}

type minerEnvBuilder struct {
	provingPeriodEnd *types.BlockHeight
	message          types.MethodID
	sectorSet        SectorSet
	sectorSize       *types.BytesAmount
	verifier         *verification.FakeVerifier
}

func (b *minerEnvBuilder) build() (*vm.FakeVMContext, *verification.FakeVerifier, *Impl) {
	minerState := NewState(address.TestAddress, address.TestAddress, peer.ID(""), b.sectorSize)
	minerState.SectorCommitments = b.sectorSet
	minerState.ProvingPeriodEnd = b.provingPeriodEnd

	if b.sectorSet == nil {
		b.sectorSet = NewSectorSet()
	}

	if b.sectorSize == nil {
		b.sectorSize = types.OneKiBSectorSize
	}

	if b.verifier == nil {
		b.verifier = &verification.FakeVerifier{}
	}

	vmctx := vm.NewFakeVMContextWithVerifier(types.NewUnsignedMessage(address.TestAddress, address.TestAddress2, 0, types.ZeroAttoFIL, b.message, nil), minerState, b.verifier)

	actor := Impl(Actor{})
	return vmctx, b.verifier, &actor
}
