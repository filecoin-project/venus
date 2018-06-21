package miner_test

import (
	"context"
	"math/big"
	"testing"

	"gx/ipfs/QmcYBp5EDnJKfVN63F71rDTksvEf1cfijwCTWtw6bPG58T/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

func createTestMiner(assert *assert.Assertions, st state.Tree, pledge, collateral int64, key []byte) types.Address {
	pdata := actor.MustConvertParams(types.NewBytesAmount(10000), key)
	nonce := core.MustGetNonce(st, address.TestAddress)
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, nonce, types.NewAttoFILFromFIL(100), "createMiner", pdata)

	result, err := core.ApplyMessage(context.Background(), st, msg, types.NewBlockHeight(0))
	assert.NoError(err)

	addr, err := types.NewAddressFromBytes(result.Receipt.Return[0])
	assert.NoError(err)
	return addr
}

func TestAddAsk(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := core.InitGenesis(cst)
	assert.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	assert.NoError(err)

	outAddr := createTestMiner(assert, st, 10000, 500, []byte{})

	// make an ask, and then make sure it all looks good
	pdata := actor.MustConvertParams(types.NewAttoFILFromFIL(100), types.NewBytesAmount(150))
	msg := types.NewMessage(address.TestAddress, outAddr, 1, nil, "addAsk", pdata)

	result, err := core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Equal(0, big.NewInt(0).Cmp(big.NewInt(0).SetBytes(result.Receipt.Return[0])))

	storageMkt, err := st.GetActor(ctx, address.StorageMarketAddress)
	assert.NoError(err)

	var strgMktStorage storagemarket.Storage
	assert.NoError(actor.UnmarshalStorage(storageMkt.ReadStorage(), &strgMktStorage))
	assert.Len(strgMktStorage.Orderbook.Asks, 1)
	assert.Equal(outAddr, strgMktStorage.Orderbook.Asks[0].Owner)

	miner, err := st.GetActor(ctx, outAddr)
	assert.NoError(err)

	var minerStorage Storage
	assert.NoError(actor.UnmarshalStorage(miner.ReadStorage(), &minerStorage))
	assert.Equal(types.NewBytesAmount(150), minerStorage.LockedStorage)

	// make another ask!
	pdata = actor.MustConvertParams(types.NewAttoFILFromFIL(110), types.NewBytesAmount(200))
	msg = types.NewMessage(address.TestAddress, outAddr, 2, nil, "addAsk", pdata)

	result, err = core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Equal(big.NewInt(1), big.NewInt(0).SetBytes(result.Receipt.Return[0]))

	storageMkt, err = st.GetActor(ctx, address.StorageMarketAddress)
	assert.NoError(err)

	var strgMktStorage2 storagemarket.Storage
	assert.NoError(actor.UnmarshalStorage(storageMkt.ReadStorage(), &strgMktStorage2))
	assert.Len(strgMktStorage2.Orderbook.Asks, 2)
	assert.Equal(outAddr, strgMktStorage2.Orderbook.Asks[1].Owner)

	miner, err = st.GetActor(ctx, outAddr)
	assert.NoError(err)

	var minerStorage2 Storage
	assert.NoError(actor.UnmarshalStorage(miner.ReadStorage(), &minerStorage2))
	assert.Equal(types.NewBytesAmount(350), minerStorage2.LockedStorage)

	// now try to create an ask larger than our pledge
	pdata = actor.MustConvertParams(big.NewInt(55), types.NewBytesAmount(9900))
	msg = types.NewMessage(address.TestAddress, outAddr, 3, nil, "addAsk", pdata)

	result, err = core.ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Contains(result.ExecutionError.Error(), Errors[ErrInsufficientPledge].Error())
}

func TestGetKey(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := core.InitGenesis(cst)
	assert.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	assert.NoError(err)

	signature := []byte("my public key")
	outAddr := createTestMiner(assert, st, 10000, 500, signature)

	// retrieve key
	msg := types.NewMessage(address.TestAddress, outAddr, 0, nil, "getKey", []byte{})

	result, exitCode, err := core.ApplyQueryMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Equal(uint8(0), exitCode)
	assert.Equal(result[0], signature)
}
