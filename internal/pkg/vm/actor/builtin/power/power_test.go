package power_test

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/power"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestPowerCreateStorageMiner(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := th.RequireCreateStorages(ctx, t)

	pid := th.RequireRandomPeerID(t)
	pdata := actor.MustConvertParams(address.TestAddress, address.TestAddress, pid, types.OneKiBSectorSize)
	msg := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, types.NewAttoFILFromFIL(100), power.CreateStorageMiner, pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)
	require.Nil(t, result.ExecutionError)

	outAddr, err := address.NewFromBytes(result.Receipt.Return[0])
	require.NoError(t, err)

	idAddr := th.RequireActorIDAddress(ctx, t, st, vms, outAddr)
	minerActor, err := st.GetActor(ctx, idAddr)
	require.NoError(t, err)

	powerAct, err := st.GetActor(ctx, address.PowerAddress)
	require.NoError(t, err)

	assert.Equal(t, types.NewAttoFILFromFIL(0), powerAct.Balance)
	assert.Equal(t, types.NewAttoFILFromFIL(100), minerActor.Balance)

	var mstor miner.State
	builtin.RequireReadState(t, vms, idAddr, minerActor, &mstor)

	assert.Equal(t, mstor.ActiveCollateral, types.NewAttoFILFromFIL(0))
	assert.Equal(t, mstor.PeerID, pid)

}

func TestProcessPowerReport(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hundredAttoFIL := types.NewAttoFILFromFIL(100)

	t.Run("happy path", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)
		pid := th.RequireRandomPeerID(t)
		outAddr := requireCreateMiner(hundredAttoFIL, t, st, vms, address.TestAddress, pid, 0)

		minerAddr := th.RequireActorIDAddress(ctx, t, st, vms, outAddr)
		reportInit := requireGetPowerReport(t, st, vms, minerAddr)
		assert.Equal(t, types.NewBytesAmount(0), reportInit.ActivePower)
		assert.Equal(t, types.NewBytesAmount(0), reportInit.InactivePower)

		// set power
		report1 := types.NewPowerReport(600, 5555)
		pdata1 := actor.MustConvertParams(report1, minerAddr)
		msg1 := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, hundredAttoFIL, power.ProcessPowerReport, pdata1)
		result1, err := th.ApplyTestMessage(st, vms, msg1, types.NewBlockHeight(0))
		require.NoError(t, err)
		require.Nil(t, result1.ExecutionError)

		report1Out := requireGetPowerReport(t, st, vms, minerAddr)
		assert.Equal(t, report1.ActivePower, report1Out.ActivePower)
		assert.Equal(t, report1.InactivePower, report1Out.InactivePower)

		// set power again
		report2 := types.NewPowerReport(77, 990)
		pdata2 := actor.MustConvertParams(report2, minerAddr)
		msg2 := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, hundredAttoFIL, power.ProcessPowerReport, pdata2)
		result2, err := th.ApplyTestMessage(st, vms, msg2, types.NewBlockHeight(0))
		require.NoError(t, err)
		require.Nil(t, result2.ExecutionError)

		report2Out := requireGetPowerReport(t, st, vms, minerAddr)
		assert.Equal(t, report2.ActivePower, report2Out.ActivePower)
		assert.Equal(t, report2.InactivePower, report2Out.InactivePower)

	})

	t.Run("process nonexistent account", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)
		report := types.NewPowerReport(1, 2)
		pdata := actor.MustConvertParams(report, address.TestAddress)
		msg := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, hundredAttoFIL, power.ProcessPowerReport, pdata)
		result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
		assert.NoError(t, err)
		assert.Equal(t, power.Errors[power.ErrUnknownEntry], result.ExecutionError)
	})
}

func TestGetTotalPower(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hundredAttoFIL := types.NewAttoFILFromFIL(100)

	st, vms := th.RequireCreateStorages(ctx, t)
	pid := th.RequireRandomPeerID(t)
	outAddr := requireCreateMiner(hundredAttoFIL, t, st, vms, address.TestAddress, pid, 0)

	minerAddr := th.RequireActorIDAddress(ctx, t, st, vms, outAddr)

	// set power
	report1 := types.NewPowerReport(600, 400)
	pdata1 := actor.MustConvertParams(report1, minerAddr)
	msg1 := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, hundredAttoFIL, power.ProcessPowerReport, pdata1)
	result1, err := th.ApplyTestMessage(st, vms, msg1, types.NewBlockHeight(0))
	require.NoError(t, err)
	require.Nil(t, result1.ExecutionError)

	// get power
	msg2 := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, hundredAttoFIL, power.GetTotalPower, nil)
	result2, err := th.ApplyTestMessage(st, vms, msg2, types.NewBlockHeight(1))
	require.NoError(t, err)
	require.Equal(t, 1, len(result2.Receipt.Return))
	totalPowerIface, err := abi.Deserialize(result2.Receipt.Return[0], abi.BytesAmount)
	require.NoError(t, err)
	totalPower, ok := totalPowerIface.Val.(*types.BytesAmount)
	require.True(t, ok)
	assert.Equal(t, types.NewBytesAmount(1000), totalPower)
}

func TestRemoveStorageMiner(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hundredAttoFIL := types.NewAttoFILFromFIL(100)
	t.Run("happy path", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)
		pid := th.RequireRandomPeerID(t)
		outAddr := requireCreateMiner(hundredAttoFIL, t, st, vms, address.TestAddress, pid, 0)

		minerAddr := th.RequireActorIDAddress(ctx, t, st, vms, outAddr)
		pdata := actor.MustConvertParams(minerAddr)
		msg := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, hundredAttoFIL, power.RemoveStorageMiner, pdata)
		result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
		assert.NoError(t, err)
		assert.Nil(t, result.ExecutionError)

		// power actor no longer provides access to this entry
		assertEntryNotFound(t, st, vms, minerAddr)
	})

	t.Run("remove nonempty fails", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)
		pid := th.RequireRandomPeerID(t)
		outAddr := requireCreateMiner(hundredAttoFIL, t, st, vms, address.TestAddress, pid, 0)

		minerAddr := th.RequireActorIDAddress(ctx, t, st, vms, outAddr)

		// set power
		report1 := types.NewPowerReport(600, 400)
		pdata1 := actor.MustConvertParams(report1, minerAddr)
		msg1 := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, hundredAttoFIL, power.ProcessPowerReport, pdata1)
		result1, err := th.ApplyTestMessage(st, vms, msg1, types.NewBlockHeight(0))
		require.NoError(t, err)
		require.Nil(t, result1.ExecutionError)

		pdata2 := actor.MustConvertParams(minerAddr)
		msg2 := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, hundredAttoFIL, power.RemoveStorageMiner, pdata2)
		result2, err := th.ApplyTestMessage(st, vms, msg2, types.NewBlockHeight(0))
		require.NoError(t, err)
		assert.Equal(t, power.Errors[power.ErrDeleteMinerWithPower], result2.ExecutionError)

		// power actor still provides access to this entry
		finalReport := requireGetPowerReport(t, st, vms, minerAddr)
		assert.Equal(t, report1.ActivePower, finalReport.ActivePower)
		assert.Equal(t, report1.InactivePower, finalReport.InactivePower)
	})

	t.Run("remove nonexistent fails", func(t *testing.T) {
		st, vms := th.RequireCreateStorages(ctx, t)
		pdata := actor.MustConvertParams(address.TestAddress)
		msg := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, hundredAttoFIL, power.RemoveStorageMiner, pdata)
		result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
		require.NoError(t, err)
		assert.Equal(t, power.Errors[power.ErrUnknownEntry], result.ExecutionError)
	})
}

// helpers
func requireCreateMiner(collateral types.AttoFIL, t *testing.T, st state.Tree, vms vm.StorageMap, minerOwnerAddr address.Address, pid peer.ID, height uint64) address.Address {
	pdata := actor.MustConvertParams(minerOwnerAddr, minerOwnerAddr, pid, types.OneKiBSectorSize)
	msg := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, collateral, power.CreateStorageMiner, pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)
	require.Nil(t, result.ExecutionError)

	minerAddr, err := address.NewFromBytes(result.Receipt.Return[0])
	require.NoError(t, err)
	return minerAddr
}

func requireGetPowerReport(t *testing.T, st state.Tree, vms vm.StorageMap, minerAddr address.Address) types.PowerReport {
	pdata := actor.MustConvertParams(minerAddr)
	msg := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, types.NewAttoFILFromFIL(100), power.GetPowerReport, pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)
	require.Nil(t, result.ExecutionError)
	require.Equal(t, 1, len(result.Receipt.Return))

	var report types.PowerReport
	require.NoError(t, actor.UnmarshalStorage(result.Receipt.Return[0], &report))
	return report
}

func assertEntryNotFound(t *testing.T, st state.Tree, vms vm.StorageMap, minerAddr address.Address) {
	// assertEntryNotFound attempts to get the power report at the provided
	// and asserts that the getPowerReport message returns an execution error
	// because the power table entry is not found
	pdata := actor.MustConvertParams(minerAddr)
	msg := types.NewUnsignedMessage(address.TestAddress, address.PowerAddress, 0, types.NewAttoFILFromFIL(100), power.GetPowerReport, pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)
	assert.Equal(t, power.Errors[power.ErrUnknownEntry], result.ExecutionError)
}
