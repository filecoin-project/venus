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

func TestStorageMarketGetMiners(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	st, vms := th.RequireCreateStorages(ctx, t)

	addrs := *assertGetMiners(t, st, vms)
	assert.Len(t, addrs, 0)

	expected := []address.Address{
		mustCreateStorageMiner(t, st, vms, 0),
		mustCreateStorageMiner(t, st, vms, 1),
		mustCreateStorageMiner(t, st, vms, 2),
	}

	addrs = *assertGetMiners(t, st, vms)
	assert.Len(t, addrs, 3)

	assertEqualAddrs(t, expected, addrs)
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

// mustCreateStorageMiner creates a storage miner for the given state tree & storage map
// and returns the address of the created miner.
func mustCreateStorageMiner(t *testing.T, st state.Tree, vms vm.StorageMap, height uint64) address.Address {
	pid := th.RequireRandomPeerID(t)
	pdata := actor.MustConvertParams(types.OneKiBSectorSize, pid)
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(100), "createStorageMiner", pdata)

	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(height))
	require.NoError(t, err)
	require.Nil(t, result.ExecutionError)
	minerAddr, err := address.NewFromBytes(result.Receipt.Return[0])
	require.NoError(t, err)
	require.NotNil(t, minerAddr)
	return minerAddr
}

// assertGetMiners calls "getMiners" message / method, deserializes the result and returns the
// addresses of miners in storage
func assertGetMiners(t *testing.T, st state.Tree, vms vm.StorageMap) *[]address.Address {
	res, err := th.CreateAndApplyTestMessage(t, st, vms, address.StorageMarketAddress, 0, 0, "getMiners", nil)
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)
	assert.Equal(t, uint8(0), res.Receipt.ExitCode)

	dsz, err := abi.Deserialize(res.Receipt.Return[0], abi.Addresses)
	require.NoError(t, err)
	addrs := dsz.Val.(*[]address.Address)
	return addrs
}

// assertEqualAddrs ensures the "expected" array of addresses is value-equal to "actual"
// we're not going to be doing this for large arrays so it doesn't need to be efficient.
func assertEqualAddrs(t *testing.T, expected, actual []address.Address) {
	require.Equal(t, len(expected), len(actual))
	for _, el := range expected {
		assert.True(t, indexOfAddr(t, el, actual) >= 0)
	}
}

// indexOfAddr, a standard indexOf func for a slice of address.Address
func indexOfAddr(t *testing.T, addr address.Address, addrs []address.Address) int {
	for i, el := range addrs {
		if addr.String() == el.String() {
			return i
		}
	}
	return -1
}
