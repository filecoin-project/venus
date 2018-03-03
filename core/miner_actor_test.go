package core

import (
	"context"
	"math/big"
	"testing"

	"gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func createTestMiner(assert *assert.Assertions, st types.StateTree, pledge, collateral int64) types.Address {
	pdata := mustConvertParams(big.NewInt(10000))
	msg := types.NewMessage(TestAccount, StorageMarketAddress, big.NewInt(100), "createMiner", pdata)

	receipt, err := ApplyMessage(context.Background(), st, msg)
	assert.NoError(err)

	return types.Address(receipt.Return)
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
	pdata := mustConvertParams(big.NewInt(100), big.NewInt(150))
	msg := types.NewMessage(TestAccount, outAddr, nil, "addAsk", pdata)

	receipt, err := ApplyMessage(ctx, st, msg)
	assert.NoError(err)
	assert.Equal(big.NewInt(0), big.NewInt(0).SetBytes(receipt.Return))

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
	assert.Equal(big.NewInt(150), minerStorage.LockedStorage)

	// make another ask!
	pdata = mustConvertParams(big.NewInt(110), big.NewInt(200))
	msg = types.NewMessage(TestAccount, outAddr, nil, "addAsk", pdata)

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
	assert.Equal(big.NewInt(350), minerStorage2.LockedStorage)

	// now try to create an ask larger than our pledge
	pdata = mustConvertParams(big.NewInt(55), big.NewInt(9900))
	msg = types.NewMessage(TestAccount, outAddr, nil, "addAsk", pdata)

	_, err = ApplyMessage(ctx, st, msg)
	assert.EqualError(err, "failed to send message: not enough pledged storage for new ask")
}
