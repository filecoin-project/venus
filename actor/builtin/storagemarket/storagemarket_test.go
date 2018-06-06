package storagemarket_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"
	"testing"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageMarketCreateMiner(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := core.InitGenesis(cst)
	assert.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	assert.NoError(err)

	pdata := actor.MustConvertParams(types.NewBytesAmount(10000), []byte{})
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewTokenAmount(100), "createMiner", pdata)
	result, err := core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)

	outAddr, err := types.NewAddressFromBytes(result.Receipt.Return[0])
	assert.NoError(err)
	minerActor, err := st.GetActor(ctx, outAddr)
	assert.NoError(err)

	storageMkt, err := st.GetActor(ctx, address.StorageMarketAddress)
	assert.NoError(err)

	assert.Equal(types.NewTokenAmount(0), storageMkt.Balance)
	assert.Equal(types.NewTokenAmount(100), minerActor.Balance)

	var mstor miner.Storage
	assert.NoError(cbor.DecodeInto(minerActor.ReadStorage(), &mstor))

	assert.Equal(mstor.Collateral, types.NewTokenAmount(100))
	assert.Equal(mstor.PledgeBytes, types.NewBytesAmount(10000))
}

func TestStorageMarketCreateMinerPledgeTooLow(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := core.InitGenesis(cst)
	assert.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	assert.NoError(err)

	pdata := actor.MustConvertParams(types.NewBytesAmount(50), []byte{})
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewTokenAmount(100), "createMiner", pdata)
	result, err := core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Contains(result.ExecutionError.Error(), ErrPledgeTooLow.Error())
}

func TestStorageMarkeCreateMinerDoesNotOverwriteActorBalance(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := core.InitGenesis(cst)
	assert.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	assert.NoError(err)

	// create account of future miner actor by sending FIL to the predicted address
	minerAddr, err := deriveMinerAddress(address.TestAddress, 0)
	require.NoError(err)

	msg := types.NewMessage(address.TestAddress2, minerAddr, 0, types.NewTokenAmount(100), "", []byte{})
	result, err := core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	require.NoError(err)
	require.Equal(uint8(0), result.Receipt.ExitCode)

	pdata := actor.MustConvertParams(types.NewBytesAmount(15000), []byte{})
	msg = types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewTokenAmount(200), "createMiner", pdata)
	result, err = core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	require.NoError(err)
	require.Equal(uint8(0), result.Receipt.ExitCode)

	// ensure our derived address is the address storage market creates
	createdAddress, err := types.NewAddressFromBytes(result.Receipt.Return[0])
	require.NoError(err)
	assert.Equal(minerAddr, createdAddress)

	miner, err := st.GetActor(ctx, minerAddr)
	require.NoError(err)

	// miner balance should be sum of messages
	assert.Equal(types.NewTokenAmount(300), miner.Balance)
}

func TestStorageMarkeCreateMinerErrorsOnInvalidKey(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := core.InitGenesis(cst)
	require.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	require.NoError(err)

	publicKey := []byte("012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567")
	pdata := actor.MustConvertParams(types.NewBytesAmount(15000), publicKey)
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewTokenAmount(200), "createMiner", pdata)
	result, err := core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	require.NoError(err)
	assert.Contains(result.ExecutionError.Error(), miner.ErrPublicKeyTooBig.Error())
}

func TestStorageMarketAddBid(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := core.InitGenesis(cst)
	assert.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	assert.NoError(err)

	// create a bid
	pdata := actor.MustConvertParams(types.NewTokenAmount(20), types.NewBytesAmount(30))
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewTokenAmount(600), "addBid", pdata)
	result, err := core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)

	assert.Equal(uint8(0), result.Receipt.ExitCode)
	assert.Equal(types.NewTokenAmount(0), types.NewTokenAmountFromBytes(result.Receipt.Return[0]))

	// create another bid
	pdata = actor.MustConvertParams(types.NewTokenAmount(15), types.NewBytesAmount(80))
	msg = types.NewMessage(address.TestAddress, address.StorageMarketAddress, 1, types.NewTokenAmount(1200), "addBid", pdata)
	result, err = core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)

	assert.Equal(uint8(0), result.Receipt.ExitCode)
	assert.Equal(types.NewTokenAmount(1), types.NewTokenAmountFromBytes(result.Receipt.Return[0]))

	// try to create a bid, but send wrong value
	pdata = actor.MustConvertParams(types.NewTokenAmount(90), types.NewBytesAmount(100))
	msg = types.NewMessage(address.TestAddress, address.StorageMarketAddress, 2, types.NewTokenAmount(600), "addBid", pdata)
	result, err = core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Contains(result.ExecutionError.Error(), "must send price * size funds to create bid")
}

func TestStorageMarketMakeDeal(t *testing.T) {
	// TODO: add test cases for:
	// - ask too small
	// - not enough collateral
	// - bid already used
	// - multiple bids, one ask
	// - cases where ask.price != bid.price (above and below)
	// - bad 'signature'
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := core.InitGenesis(cst)
	assert.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	assert.NoError(err)

	// create a bid
	pdata := actor.MustConvertParams(types.NewTokenAmount(20), types.NewBytesAmount(30))
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewTokenAmount(600), "addBid", pdata)
	result, err := core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)

	assert.Equal(uint8(0), result.Receipt.ExitCode)
	assert.Equal(types.NewTokenAmount(0), types.NewTokenAmountFromBytes(result.Receipt.Return[0]))

	// create a miner
	minerAddr := createTestMiner(assert, st, 50000, 45000)

	// add an ask on it
	pdata = actor.MustConvertParams(types.NewTokenAmount(25), types.NewBytesAmount(35))
	nonce := core.MustGetNonce(st, address.TestAddress)
	msg = types.NewMessage(address.TestAddress, minerAddr, nonce, nil, "addAsk", pdata)
	result, err = core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Equal(uint8(0), result.Receipt.ExitCode)

	// now make a deal
	ref := types.NewCidForTestGetter()()
	sig := address.TestAddress.Bytes()
	pdata = actor.MustConvertParams(big.NewInt(0), big.NewInt(0), sig, ref.Bytes()) // askID, bidID, signature, datacid
	nonce = core.MustGetNonce(st, address.TestAddress)
	msg = types.NewMessage(address.TestAddress, address.StorageMarketAddress, nonce, nil, "addDeal", pdata)
	result, err = core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Equal(uint8(0), result.Receipt.ExitCode)

	sma, err := st.GetActor(ctx, address.StorageMarketAddress)
	assert.NoError(err)
	var sms Storage
	assert.NoError(actor.UnmarshalStorage(sma.ReadStorage(), &sms))
	assert.Len(sms.Filemap.Deals, 1)
	assert.Equal("5", sms.Orderbook.Asks[0].Size.String())
}

// this is used to simulate an attack where someone derives the likely address of another miner's
// minerActor and sends some FIL. If that FIL creates an actor tha cannot be upgraded to a miner
// actor, this action will block the other user. Another possibility is that the miner actor will
// overwrite the account with the balance thereby obliterating the FIL.
func deriveMinerAddress(creator types.Address, nonce uint64) (types.Address, error) {
	buf := new(bytes.Buffer)

	if _, err := buf.Write(creator.Bytes()); err != nil {
		return types.Address{}, err
	}

	if err := binary.Write(buf, binary.BigEndian, nonce); err != nil {
		return types.Address{}, err
	}

	hash, err := types.AddressHash(buf.Bytes())
	if err != nil {
		return types.Address{}, err
	}

	return types.NewMainnetAddress(hash), nil
}

// TODO: deduplicate with code in miner/miner_test.go
func createTestMiner(assert *assert.Assertions, st state.Tree, pledge, collateral int64) types.Address {
	pdata := actor.MustConvertParams(types.NewBytesAmount(10000), []byte{})
	nonce := core.MustGetNonce(st, address.TestAddress)
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, nonce, types.NewTokenAmount(100), "createMiner", pdata)

	result, err := core.ApplyMessage(context.Background(), st, msg, types.NewBlockHeight(0))
	assert.NoError(err)

	addr, err := types.NewAddressFromBytes(result.Receipt.Return[0])
	assert.NoError(err)
	return addr
}
