package storagemarket_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageMarketCreateStorageMiner(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	pid := th.RequireRandomPeerID(t)
	pdata := actor.MustConvertParams([]byte{}, big.NewInt(10), pid)
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

	assert.Equal(t, mstor.Collateral, types.NewAttoFILFromFIL(100))
	assert.Equal(t, mstor.PledgeSectors, big.NewInt(10))
	assert.Equal(t, mstor.PeerID, pid)
}

func TestStorageMarketCreateStorageMinerPledgeTooLow(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pledge := big.NewInt(5)
	st, vms := core.CreateStorages(ctx, t)
	pdata := actor.MustConvertParams([]byte{}, pledge, th.RequireRandomPeerID(t))
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, MinimumCollateral(pledge), "createStorageMiner", pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))

	assert.NoError(t, err)
	require.NotNil(t, result.ExecutionError)
	assert.Contains(t, result.ExecutionError.Error(), Errors[ErrPledgeTooLow].Error())
}

func TestStorageMarketCreateStorageMinerInsufficientCollateral(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)
	pdata := actor.MustConvertParams([]byte{}, big.NewInt(15000), th.RequireRandomPeerID(t))
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(14), "createStorageMiner", pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))

	assert.NoError(t, err)
	require.NotNil(t, result.ExecutionError)
	assert.Contains(t, result.ExecutionError.Error(), Errors[ErrInsufficientCollateral].Error())
}

func TestStorageMarkeCreateStorageMinerDoesNotOverwriteActorBalance(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	// create account of future miner actor by sending FIL to the predicted address
	minerAddr, err := deriveMinerAddress(address.TestAddress, 0)
	require.NoError(t, err)

	msg := types.NewMessage(address.TestAddress2, minerAddr, 0, types.NewAttoFILFromFIL(100), "", []byte{})
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)
	require.Equal(t, uint8(0), result.Receipt.ExitCode)

	pdata := actor.MustConvertParams([]byte{}, big.NewInt(15), th.RequireRandomPeerID(t))
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
	assert.Equal(t, types.NewAttoFILFromFIL(300), miner.Balance)
}

func TestStorageMarkeCreateStorageMinerErrorsOnInvalidKey(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	publicKey := []byte("012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567")
	pdata := actor.MustConvertParams(publicKey, big.NewInt(15), th.RequireRandomPeerID(t))

	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(200), "createStorageMiner", pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)
	assert.Contains(t, result.ExecutionError.Error(), miner.Errors[miner.ErrPublicKeyTooBig].Error())
}

func TestMinimumCollateral(t *testing.T) {
	tf.UnitTest(t)

	numSectors := big.NewInt(25000)
	expected := types.NewAttoFILFromFIL(25)
	assert.Equal(t, MinimumCollateral(numSectors), expected)
}

func TestProofsMode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)
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
