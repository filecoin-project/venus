package core

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTotal(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	power := uint64(19)
	ctx, cm, _, st := requireMinerWithPower(t, power)

	actual, err := (&marketView{}).Total(ctx, st, cm.Blockstore)
	require.NoError(err)

	assert.Equal(power, actual)
}

func TestMiner(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	power := uint64(12)
	ctx, cm, addr, st := requireMinerWithPower(t, power)

	actual, err := (&marketView{}).Miner(ctx, st, cm.Blockstore, addr)
	require.NoError(err)

	assert.Equal(power, actual)
}

func requireMinerWithPower(t *testing.T, power uint64) (context.Context, *ChainManager, address.Address, state.Tree) {
	require := require.New(t)

	ctx, _, _, cm := newTestUtils()
	cm.PwrTableView = &TestView{}
	// setup miner power in genesis block
	ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
	mockSigner := types.NewMockSigner(ki)
	testAddress := mockSigner.Addresses[0]
	testGen := MakeGenesisFunc(
		ActorAccount(testAddress, types.NewAttoFILFromFIL(10000)),
	)
	require.NoError(cm.Genesis(ctx, testGen))
	genesisBlock, err := cm.FetchBlock(ctx, cm.genesisCid)
	require.NoError(err)
	addr, block, _, err := CreateMinerWithPower(ctx, t, cm, genesisBlock, mockSigner, 0, mockSigner.Addresses[0], power)
	require.NoError(err)
	st, err := cm.State(ctx, []*types.Block{block})
	require.NoError(err)
	return ctx, cm, addr, st
}
