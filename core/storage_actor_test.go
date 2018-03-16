package core

import (
	"context"
	"math/big"
	"testing"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"
)

func mustConvertParams(params ...interface{}) []byte {
	vals, err := abi.ToValues(params)
	if err != nil {
		panic(err)
	}

	out, err := abi.EncodeValues(vals)
	if err != nil {
		panic(err)
	}
	return out
}

func TestStorageMarketCreateMiner(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := InitGenesis(cst)
	assert.NoError(err)

	st, err := types.LoadStateTree(ctx, cst, blk.StateRoot)
	assert.NoError(err)

	pdata := mustConvertParams(big.NewInt(10000))
	msg := types.NewMessage(TestAccount, StorageMarketAddress, big.NewInt(100), "createMiner", pdata)
	receipt, err := ApplyMessage(ctx, st, msg)
	assert.NoError(err)

	outAddr, err := types.NewAddressFromBytes(receipt.Return)
	assert.NoError(err)
	miner, err := st.GetActor(ctx, outAddr)
	assert.NoError(err)

	storageMkt, err := st.GetActor(ctx, StorageMarketAddress)
	assert.NoError(err)

	assert.Equal(big.NewInt(0), storageMkt.Balance)
	assert.Equal(big.NewInt(100), miner.Balance)

	var mstor MinerStorage
	assert.NoError(cbor.DecodeInto(miner.ReadStorage(), &mstor))

	assert.Equal(mstor.Collateral, big.NewInt(100))
	assert.Equal(mstor.PledgeBytes, big.NewInt(10000))
}

func TestStorageMarketCreateMinerPledgeTooLow(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := InitGenesis(cst)
	assert.NoError(err)

	st, err := types.LoadStateTree(ctx, cst, blk.StateRoot)
	assert.NoError(err)

	pdata := mustConvertParams(big.NewInt(50))
	msg := types.NewMessage(TestAccount, StorageMarketAddress, big.NewInt(100), "createMiner", pdata)
	receipt, err := ApplyMessage(ctx, st, msg)
	assert.NoError(err)
	assert.Contains(receipt.Error, ErrPledgeTooLow.Error())
}

func TestStorageMarketAddBid(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := InitGenesis(cst)
	assert.NoError(err)

	st, err := types.LoadStateTree(ctx, cst, blk.StateRoot)
	assert.NoError(err)

	// create a bid
	pdata := mustConvertParams(big.NewInt(20), big.NewInt(30))
	msg := types.NewMessage(TestAccount, StorageMarketAddress, big.NewInt(600), "addBid", pdata)
	receipt, err := ApplyMessage(ctx, st, msg)
	assert.NoError(err)

	assert.Equal(uint8(0), receipt.ExitCode)
	assert.Equal(big.NewInt(0), big.NewInt(0).SetBytes(receipt.Return))

	// create another bid
	pdata = mustConvertParams(big.NewInt(15), big.NewInt(80))
	msg = types.NewMessage(TestAccount, StorageMarketAddress, big.NewInt(1200), "addBid", pdata)
	receipt, err = ApplyMessage(ctx, st, msg)
	assert.NoError(err)

	assert.Equal(uint8(0), receipt.ExitCode)
	assert.Equal(big.NewInt(1), big.NewInt(0).SetBytes(receipt.Return))

	// try to create a bid, but send wrong value
	pdata = mustConvertParams(big.NewInt(90), big.NewInt(100))
	msg = types.NewMessage(TestAccount, StorageMarketAddress, big.NewInt(600), "addBid", pdata)
	receipt, err = ApplyMessage(ctx, st, msg)
	assert.NoError(err)
	assert.Contains(receipt.Error, "must send price * size funds to create bid")
}

func TestStorageMarketMakeDeal(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := InitGenesis(cst)
	assert.NoError(err)

	st, err := types.LoadStateTree(ctx, cst, blk.StateRoot)
	assert.NoError(err)

	// create a bid
	pdata := mustConvertParams(big.NewInt(20), big.NewInt(30))
	msg := types.NewMessage(TestAccount, StorageMarketAddress, big.NewInt(600), "addBid", pdata)
	receipt, err := ApplyMessage(ctx, st, msg)
	assert.NoError(err)

	assert.Equal(uint8(0), receipt.ExitCode)
	assert.Equal(big.NewInt(0), big.NewInt(0).SetBytes(receipt.Return))

	// create a miner
	minerAddr := createTestMiner(assert, st, 50000, 45000)

	// add an ask on it
	pdata = mustConvertParams(big.NewInt(25), big.NewInt(35))
	msg = types.NewMessage(TestAccount, minerAddr, nil, "addAsk", pdata)
	receipt, err = ApplyMessage(ctx, st, msg)
	assert.NoError(err)
	assert.Equal(uint8(0), receipt.ExitCode)

	// now make a deal
	sig := TestAccount.Bytes()
	pdata = mustConvertParams(big.NewInt(0), big.NewInt(0), sig) // askID, bidID, signature
	msg = types.NewMessage(TestAccount, StorageMarketAddress, nil, "addDeal", pdata)
	receipt, err = ApplyMessage(ctx, st, msg)
	assert.NoError(err)
	assert.Equal(uint8(0), receipt.ExitCode)

	sma, err := st.GetActor(ctx, StorageMarketAddress)
	assert.NoError(err)
	var sms StorageMarketStorage
	assert.NoError(UnmarshalStorage(sma.ReadStorage(), &sms))
	assert.Len(sms.Orderbook.Deals, 1)
	assert.Equal("5", sms.Orderbook.Asks[0].Size.String())
}
