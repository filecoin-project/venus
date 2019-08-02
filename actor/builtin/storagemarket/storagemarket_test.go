package storagemarket_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageMarketCreateStorageMiner(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := th.RequireCreateStorages(ctx, t)

	pid := th.RequireRandomPeerID(t)
	pdata := actor.MustConvertParams(types.OneKiBSectorSize, pid)
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(100), "createStorageMiner", pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)
	require.Nil(t, result.ExecutionError)

	outAddr, err := address.NewFromBytes(result.Receipt.Return[0])
	require.NoError(t, err)

	minerActor, err := st.GetActor(ctx, outAddr)
	require.NoError(t, err)

	storageMkt, err := st.GetActor(ctx, address.StorageMarketAddress)
	require.NoError(t, err)

	assert.Equal(t, types.NewAttoFILFromFIL(0), storageMkt.Balance)
	assert.Equal(t, types.NewAttoFILFromFIL(100), minerActor.Balance)

	var mstor miner.State
	builtin.RequireReadState(t, vms, outAddr, minerActor, &mstor)

	assert.Equal(t, mstor.ActiveCollateral, types.NewAttoFILFromFIL(0))
	assert.Equal(t, mstor.PeerID, pid)
}

func TestStorageMarketCreateStorageMinerDoesNotOverwriteActorBalance(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := th.RequireCreateStorages(ctx, t)

	// create account of future miner actor by sending FIL to the predicted address
	minerAddr, err := deriveMinerAddress(address.TestAddress, 0)
	require.NoError(t, err)

	msg := types.NewMessage(address.TestAddress2, minerAddr, 0, types.NewAttoFILFromFIL(100), "", []byte{})
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)
	require.Equal(t, uint8(0), result.Receipt.ExitCode)

	pdata := actor.MustConvertParams(types.OneKiBSectorSize, th.RequireRandomPeerID(t))
	msg = types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(200), "createStorageMiner", pdata)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)
	require.Equal(t, uint8(0), result.Receipt.ExitCode)
	require.NoError(t, result.ExecutionError)

	// ensure our derived address is the address storage market creates
	createdAddress, err := address.NewFromBytes(result.Receipt.Return[0])
	require.NoError(t, err)
	assert.Equal(t, minerAddr, createdAddress)
	miner, err := st.GetActor(ctx, minerAddr)
	require.NoError(t, err)

	// miner balance should be sum of messages
	assert.Equal(t, types.NewAttoFILFromFIL(300).String(), miner.Balance.String())
}

func TestProofsMode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := th.RequireCreateStorages(ctx, t)
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(14), "getProofsMode", []byte{})
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))

	require.NoError(t, err)
	require.NoError(t, result.ExecutionError)

	proofsModeInterface, err := abi.Deserialize(result.Receipt.Return[0], abi.ProofsMode)
	require.NoError(t, err)

	proofsMode, ok := proofsModeInterface.Val.(types.ProofsMode)
	require.True(t, ok)

	assert.Equal(t, types.TestProofsMode, proofsMode)
}
func TestStorageMarketGetLateMiners(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	coll := types.NewAttoFILFromFIL(100)
	emptyMiners := map[string]uint64{}

	type testCase struct {
		Name           string
		BlockHeight    uint64
		ExpectedMiners map[string]uint64
	}

	t.Run("if no storage miners, empty set", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)

		tc := []testCase{
			{Name: "just after commit", BlockHeight: 2, ExpectedMiners: emptyMiners},
			{Name: "super late", BlockHeight: 2000, ExpectedMiners: emptyMiners},
		}

		for _, el := range tc {
			t.Run(el.Name, func(t *testing.T) {
				miners := *assertGetLateMiners(t, st, vms, el.BlockHeight)
				assert.Equal(t, el.ExpectedMiners, miners)
			})
		}
	})

	t.Run("if no storage miners have commitments, empty set", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)
		// create miners without commitments
		_ = []address.Address{
			th.CreateTestMinerWith(coll, t, st, vms, address.TestAddress, th.RequireRandomPeerID(t), 0),
			th.CreateTestMinerWith(coll, t, st, vms, address.TestAddress, th.RequireRandomPeerID(t), 1),
			th.CreateTestMinerWith(coll, t, st, vms, address.TestAddress, th.RequireRandomPeerID(t), 2),
		}

		tc := []testCase{
			{Name: "just after commit", BlockHeight: 2, ExpectedMiners: emptyMiners},
			{Name: "super late", BlockHeight: 2000, ExpectedMiners: emptyMiners},
		}
		for _, el := range tc {
			t.Run(el.Name, func(t *testing.T) {
				miners := *assertGetLateMiners(t, st, vms, el.BlockHeight)
				assert.Equal(t, miners, el.ExpectedMiners)
			})
		}
	})

	t.Run("gets only late miners with commitments", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)

		// create 3 bootstrap miners by passing in 0 block height, so that VerifyProof is skipped
		// Otherwise this test will fail
		addr1 := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))
		addr2 := th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))
		_ = th.CreateTestMiner(t, st, vms, address.TestAddress, th.RequireRandomPeerID(t))

		// 2 of the 3 miners make a commitment
		blockHeight := 3
		sectorID := uint64(1)
		requireMakeCommitment(t, st, vms, addr1, blockHeight, sectorID)
		sectorID = uint64(2)
		requireMakeCommitment(t, st, vms, addr2, blockHeight, sectorID)

		firstCommitBlockHeight := uint64(3)
		secondProvingPeriodStart := miner.LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight
		thirdProvingPeriodStart := 2*miner.LargestSectorSizeProvingPeriodBlocks + firstCommitBlockHeight

		tc := []testCase{
			{Name: "1st commit bh", BlockHeight: firstCommitBlockHeight, ExpectedMiners: emptyMiners},
			{Name: "2nd proving pd start", BlockHeight: secondProvingPeriodStart, ExpectedMiners: emptyMiners},
			{Name: "just after 2nd proving period start", BlockHeight: secondProvingPeriodStart + 1, ExpectedMiners: map[string]uint64{
				addr1.String(): miner.PoStStateAfterProvingPeriod,
				addr2.String(): miner.PoStStateAfterProvingPeriod},
			},
			{Name: "after 3rd proving period start", BlockHeight: thirdProvingPeriodStart + 1, ExpectedMiners: map[string]uint64{
				addr1.String(): miner.PoStStateAfterGenerationAttackThreshold,
				addr2.String(): miner.PoStStateAfterGenerationAttackThreshold},
			},
		}
		for _, el := range tc {
			t.Run(el.Name, func(t *testing.T) {
				miners := *assertGetLateMiners(t, st, vms, el.BlockHeight)
				assert.Equal(t, el.ExpectedMiners, miners)
			})
		}
	})
}

func requireMakeCommitment(t *testing.T, st state.Tree, vms vm.StorageMap, minerAddr address.Address, blockHeight int, sectorID uint64) {
	builder := chain.NewBuilder(t, address.Undef)
	head := builder.AppendManyOn(blockHeight, types.UndefTipSet)
	ancestors := builder.RequireTipSets(head.Key(), blockHeight)
	res, err := th.CreateAndApplyTestMessage(t, st, vms, minerAddr, 0, 3, "commitSector", ancestors, sectorID, th.MakeCommitment(), th.MakeCommitment(), th.MakeCommitment(), th.MakeRandomBytes(types.TwoPoRepProofPartitions.ProofLen()))
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	require.Equal(t, uint8(0), res.Receipt.ExitCode)
}

func TestUpdateStorage(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	t.Run("add storage power", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)
		// Create miner so that update can pass checks
		pid := th.RequireRandomPeerID(t)
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, pid)

		// Add update to total storage
		update := types.NewBytesAmount(uint64(3000000))
		res, err := th.CreateAndApplyTestMessageFrom(
			t,
			st,
			vms,
			minerAddr,
			address.StorageMarketAddress,
			0,
			0,
			"updateStorage",
			nil,
			update,
		)
		require.NoError(t, err)
		require.NoError(t, res.ExecutionError)
		require.Equal(t, uint8(0), res.Receipt.ExitCode)

		// Tracked storage has increased
		res, err = th.CreateAndApplyTestMessage(
			t,
			st,
			vms,
			address.StorageMarketAddress,
			0,
			0,
			"getTotalStorage",
			nil,
		)
		require.NoError(t, err)
		require.NoError(t, res.ExecutionError)
		require.Equal(t, uint8(0), res.Receipt.ExitCode)
		require.Len(t, res.Receipt.Return, 1)
		totalStorage := types.NewBytesAmountFromBytes(res.Receipt.Return[0])
		assert.True(t, update.Equal(totalStorage))
	})

	t.Run("remove storage power", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)
		// Create miner so that update can pass checks
		pid := th.RequireRandomPeerID(t)
		minerAddr := th.CreateTestMiner(t, st, vms, address.TestAddress, pid)

		// Add plus (positive number) and minus (negative number) to total storage
		plus := types.NewBytesAmount(uint64(3000000))
		minus := types.NewBytesAmount(uint64(0)).Sub(types.NewBytesAmount(uint64(1000000)))

		resPlus, err := th.CreateAndApplyTestMessageFrom(
			t,
			st,
			vms,
			minerAddr,
			address.StorageMarketAddress,
			0,
			0,
			"updateStorage",
			nil,
			plus,
		)
		require.NoError(t, err)
		require.NoError(t, resPlus.ExecutionError)
		require.Equal(t, uint8(0), resPlus.Receipt.ExitCode)

		resMinus, err := th.CreateAndApplyTestMessageFrom(
			t,
			st,
			vms,
			minerAddr,
			address.StorageMarketAddress,
			0,
			0,
			"updateStorage",
			nil,
			minus,
		)
		require.NoError(t, err)
		require.NoError(t, resMinus.ExecutionError)
		require.Equal(t, uint8(0), resMinus.Receipt.ExitCode)

		// Tracked storage is plus + minus
		res, err := th.CreateAndApplyTestMessage(
			t,
			st,
			vms,
			address.StorageMarketAddress,
			0,
			0,
			"getTotalStorage",
			nil,
		)
		require.NoError(t, err)
		require.NoError(t, res.ExecutionError)
		require.Equal(t, uint8(0), res.Receipt.ExitCode)
		require.Equal(t, 1, len(res.Receipt.Return))
		totalStorage := types.NewBytesAmountFromBytes(res.Receipt.Return[0])
		expected := plus.Add(minus)
		assert.True(t, expected.Equal(totalStorage))
	})
}

//
// Unexported test helper functions
//

// this is used to simulate an attack where someone derives the likely address of another miner's
// minerActor and sends some FIL. If that FIL creates an actor tha cannot be upgraded to a miner
// actor, this action will block the other user. Another possibility is that the miner actor will
// overwrite the account with the balance thereby obliterating the FIL.
func deriveMinerAddress(creator address.Address, nonce uint64) (address.Address, error) {
	buf := new(bytes.Buffer)

	if _, err := buf.Write(creator.Bytes()); err != nil {
		return address.Undef, err
	}

	if err := binary.Write(buf, binary.BigEndian, nonce); err != nil {
		return address.Undef, err
	}

	return address.NewActorAddress(buf.Bytes())
}

// assertGetLateMiners calls "getLateMiners" message / method, deserializes the result and returns
// a map of the late miners with their late states
func assertGetLateMiners(t *testing.T, st state.Tree, vms vm.StorageMap, height uint64) *map[string]uint64 {
	res, err := th.CreateAndApplyTestMessage(t, st, vms, address.StorageMarketAddress, 0, height, "getLateMiners", nil)
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	assert.Equal(t, uint8(0), res.Receipt.ExitCode)

	dsz, err := abi.Deserialize(res.Receipt.Return[0], abi.MinerPoStStates)
	require.NoError(t, err)
	miners := dsz.Val.(*map[string]uint64)
	return miners
}
