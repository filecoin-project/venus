package core

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTotal(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	power := uint64(987654321)
	ctx, cm, _, st := requireMinerWithPower(t, power)

	actual, err := (&marketView{}).Total(ctx, st, cm.Blockstore)
	require.NoError(err)

	assert.Equal(power, actual)
}

func TestMiner(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	power := uint64(1234567890)
	ctx, cm, addr, st := requireMinerWithPower(t, power)

	actual, err := (&marketView{}).Miner(ctx, st, cm.Blockstore, addr)
	require.NoError(err)

	assert.Equal(power, actual)
}

func requireMinerWithPower(t *testing.T, power uint64) (context.Context, *ChainManager, types.Address, state.Tree) {
	require := require.New(t)

	ctx, _, _, cm := newTestUtils()
	cm.PwrTableView = &TestView{}
	// setup miner power in genesis block
	ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
	mockSigner := types.NewMockSigner(ki)
	testAddress := mockSigner.Addresses[0]
	pwr := types.NewBytesAmount(power)
	testGen := th.MakeGenesisFunc(
		th.ActorAccount(testAddress, types.NewAttoFILFromFIL(10000)),
	)
	require.NoError(cm.Genesis(ctx, testGen))
	genesisBlock, err := cm.FetchBlock(ctx, cm.genesisCid)
	require.NoError(err)
	addr, block, _, err := CreateMinerWithPower(ctx, t, cm, genesisBlock, mockSigner, 0, mockSigner.Addresses[0], pwr)
	require.NoError(err)
	st, err := cm.State(ctx, []*types.Block{block})
	require.NoError(err)
	return ctx, cm, addr, st
}
