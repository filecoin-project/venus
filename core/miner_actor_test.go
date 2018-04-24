package core

import (
	"context"
	"math/big"
	"testing"

	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func createTestMiner(assert *assert.Assertions, st types.StateTree, pledge, collateral int64) types.Address {
	pdata := mustConvertParams(types.NewBytesAmount(10000))
	nonce := MustGetNonce(st, TestAddress)
	msg := types.NewMessage(TestAddress, StorageMarketAddress, nonce, types.NewTokenAmount(100), "createMiner", pdata)

	receipt, err := ApplyMessage(context.Background(), st, msg)
	assert.NoError(err)

	addr, err := types.NewAddressFromBytes(receipt.Return)
	assert.NoError(err)
	return addr
}

func TestAddAsk(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := InitGenesis(cst)
	assert.NoError(err)

	st, err := types.LoadStateTree(ctx, cst, blk.StateRoot)
	assert.NoError(err)

	outAddr := createTestMiner(assert, st, 10000, 500)

	// make an ask, and then make sure it all looks good
	pdata := mustConvertParams(types.NewTokenAmount(100), types.NewBytesAmount(150))
	msg := types.NewMessage(TestAddress, outAddr, 1, nil, "addAsk", pdata)

	receipt, err := ApplyMessage(ctx, st, msg)
	assert.NoError(err)
	assert.Equal(types.NewTokenAmount(0), types.NewTokenAmountFromBytes(receipt.Return))

	storageMkt, err := st.GetActor(ctx, StorageMarketAddress)
	assert.NoError(err)

	var strgMktStorage StorageMarketStorage
	assert.NoError(UnmarshalStorage(storageMkt.ReadStorage(), &strgMktStorage))
	assert.Len(strgMktStorage.Orderbook.Asks, 1)
	assert.Equal(outAddr, strgMktStorage.Orderbook.Asks[0].Owner)

	miner, err := st.GetActor(ctx, outAddr)
	assert.NoError(err)

	var minerStorage MinerStorage
	assert.NoError(UnmarshalStorage(miner.ReadStorage(), &minerStorage))
	assert.Equal(types.NewBytesAmount(150), minerStorage.LockedStorage)

	// make another ask!
	pdata = mustConvertParams(types.NewTokenAmount(110), types.NewBytesAmount(200))
	msg = types.NewMessage(TestAddress, outAddr, 2, nil, "addAsk", pdata)

	receipt, err = ApplyMessage(ctx, st, msg)
	assert.NoError(err)
	assert.Equal(big.NewInt(1), big.NewInt(0).SetBytes(receipt.Return))

	storageMkt, err = st.GetActor(ctx, StorageMarketAddress)
	assert.NoError(err)

	var strgMktStorage2 StorageMarketStorage
	assert.NoError(UnmarshalStorage(storageMkt.ReadStorage(), &strgMktStorage2))
	assert.Len(strgMktStorage2.Orderbook.Asks, 2)
	assert.Equal(outAddr, strgMktStorage2.Orderbook.Asks[1].Owner)

	miner, err = st.GetActor(ctx, outAddr)
	assert.NoError(err)

	var minerStorage2 MinerStorage
	assert.NoError(UnmarshalStorage(miner.ReadStorage(), &minerStorage2))
	assert.Equal(types.NewBytesAmount(350), minerStorage2.LockedStorage)

	// now try to create an ask larger than our pledge
	pdata = mustConvertParams(big.NewInt(55), types.NewBytesAmount(9900))
	msg = types.NewMessage(TestAddress, outAddr, 3, nil, "addAsk", pdata)

	receipt, err = ApplyMessage(ctx, st, msg)
	assert.NoError(err)
	assert.Contains(receipt.Error, ErrInsufficientPledge.Error())
}
