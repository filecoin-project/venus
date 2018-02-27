package core

import (
	"context"
	"math/big"
	"testing"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestStorageMarketCreateMiner(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cst := hamt.NewCborStore()
	blk, err := InitGenesis(cst)
	assert.NoError(err)

	msg := types.NewMessage(testAccount, StorageMarketAddress, big.NewInt(100), "createMiner", []interface{}{big.NewInt(1000)})

	st, err := types.LoadStateTree(ctx, cst, blk.StateRoot)
	assert.NoError(err)

	receipt, err := ApplyMessage(ctx, st, msg)
	assert.NoError(err)

	outAddr := types.Address(receipt.Return)
	miner, err := st.GetActor(ctx, outAddr)
	assert.NoError(err)

	assert.Equal(miner.Balance, big.NewInt(100))

	var mstor MinerStorage
	assert.NoError(cbor.DecodeInto(miner.ReadStorage(), &mstor))

	assert.Equal(mstor.Collateral, big.NewInt(100))
	assert.Equal(mstor.Pledge, big.NewInt(1000))
}
