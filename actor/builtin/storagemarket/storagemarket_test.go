package storagemarket_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestStorageMarketCreateMiner(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	pid := th.RequireRandomPeerID()
	pdata := actor.MustConvertParams(big.NewInt(10), []byte{}, pid)
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(100), "createMiner", pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)
	require.Nil(result.ExecutionError)

	outAddr, err := address.NewFromBytes(result.Receipt.Return[0])
	require.NoError(err)

	minerActor, err := st.GetActor(ctx, outAddr)
	require.NoError(err)

	storageMkt, err := st.GetActor(ctx, address.StorageMarketAddress)
	require.NoError(err)

	assert.Equal(types.NewAttoFILFromFIL(0), storageMkt.Balance)
	assert.Equal(types.NewAttoFILFromFIL(100), minerActor.Balance)

	var mstor miner.State
	builtin.RequireReadState(t, vms, outAddr, minerActor, &mstor)

	assert.Equal(mstor.Collateral, types.NewAttoFILFromFIL(100))
	assert.Equal(mstor.PledgeSectors, big.NewInt(10))
	assert.Equal(mstor.PeerID, pid)
}

func TestStorageMarketCreateMinerPledgeTooLow(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pledge := big.NewInt(5)
	st, vms := core.CreateStorages(ctx, t)
	pdata := actor.MustConvertParams(pledge, []byte{}, th.RequireRandomPeerID())
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, MinimumCollateral(pledge), "createMiner", pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))

	assert.NoError(err)
	require.NotNil(result.ExecutionError)
	assert.Contains(result.ExecutionError.Error(), Errors[ErrPledgeTooLow].Error())
}

func TestStorageMarketCreateMinerInsufficientCollateral(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)
	pdata := actor.MustConvertParams(big.NewInt(15000), []byte{}, th.RequireRandomPeerID())
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(14), "createMiner", pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))

	assert.NoError(err)
	require.NotNil(result.ExecutionError)
	assert.Contains(result.ExecutionError.Error(), Errors[ErrInsufficientCollateral].Error())
}

func TestStorageMarkeCreateMinerDoesNotOverwriteActorBalance(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	// create account of future miner actor by sending FIL to the predicted address
	minerAddr, err := deriveMinerAddress(address.TestAddress, 0)
	require.NoError(err)

	msg := types.NewMessage(address.TestAddress2, minerAddr, 0, types.NewAttoFILFromFIL(100), "", []byte{})
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)
	require.Equal(uint8(0), result.Receipt.ExitCode)

	pdata := actor.MustConvertParams(big.NewInt(15), []byte{}, th.RequireRandomPeerID())
	msg = types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(200), "createMiner", pdata)
	result, err = th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)
	require.Equal(uint8(0), result.Receipt.ExitCode)
	require.NoError(result.ExecutionError)

	// ensure our derived address is the address storage market creates
	createdAddress, err := address.NewFromBytes(result.Receipt.Return[0])
	require.NoError(err)
	assert.Equal(minerAddr, createdAddress)
	miner, err := st.GetActor(ctx, minerAddr)
	require.NoError(err)

	// miner balance should be sum of messages
	assert.Equal(types.NewAttoFILFromFIL(300), miner.Balance)
}

func TestStorageMarkeCreateMinerErrorsOnInvalidKey(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	publicKey := []byte("012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567")
	pdata := actor.MustConvertParams(big.NewInt(15), publicKey, th.RequireRandomPeerID())

	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(200), "createMiner", pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)
	assert.Contains(result.ExecutionError.Error(), miner.Errors[miner.ErrPublicKeyTooBig].Error())
}

func TestMinimumCollateral(t *testing.T) {
	assert := assert.New(t)
	numSectors := big.NewInt(25000)
	expected := types.NewAttoFILFromFIL(25)
	assert.Equal(MinimumCollateral(numSectors), expected)
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
