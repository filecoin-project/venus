package commands_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestMinerCreateIntegration(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	nodes, cancel := test.MustCreateNodesWithBootstrap(ctx, t, 1)
	defer cancel()

	newMiner := nodes[1]

	defaultAddr := newMiner.Repo.Config().Wallet.DefaultAddress
	peer := newMiner.Network().Network.GetPeerID()

	minerAddr, err := newMiner.PorcelainAPI.MinerCreate(ctx, defaultAddr, types.NewAttoFILFromFIL(1), 10000, 2048, peer, types.NewAttoFILFromFIL(1))
	require.NoError(t, err)

	tsk := newMiner.Chain().ChainReader.GetHead()
	status, err := newMiner.PorcelainAPI.MinerGetStatus(ctx, *minerAddr, tsk)
	require.NoError(t, err)

	view, err := newMiner.Chain().ActorState.StateView(tsk)
	require.NoError(t, err)
	resolvedDefaultAddress, err := view.InitResolveAddress(ctx, defaultAddr)
	require.NoError(t, err)

	assert.Equal(t, resolvedDefaultAddress, status.OwnerAddress)
}

func TestSetPrice(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	nodes, cancel := test.MustCreateNodesWithBootstrap(ctx, t, 0)
	defer cancel()

	err := nodes[0].StorageProtocol.StorageProvider.AddAsk(abi.NewTokenAmount(1000), abi.ChainEpoch(400))
	require.NoError(t, err)

	minerAddr, err := nodes[0].BlockMining.BlockMiningAPI.MinerAddress()
	require.NoError(t, err)

	asks := nodes[0].StorageProtocol.StorageProvider.ListAsks(minerAddr)
	require.Len(t, asks, 1)
	assert.Equal(t, abi.NewTokenAmount(1000), asks[0].Ask.Price)
	assert.Equal(t, abi.ChainEpoch(400), asks[0].Ask.Expiry)
}
