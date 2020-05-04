package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commands "github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestMinerCreateIntegration(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, cancel1 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel1()

	nodes, cancel2 := test.MustCreateNodesWithBootstrap(ctx, t, 1)
	defer cancel2()

	newMiner := nodes[1]

	env := commands.CreateServerEnv(ctx, newMiner)
	porcelainAPI := commands.GetPorcelainAPI(env)

	defaultAddr := newMiner.Repo.Config().Wallet.DefaultAddress
	peer := newMiner.Network().Network.GetPeerID()

	minerAddr, err := porcelainAPI.MinerCreate(ctx, defaultAddr, types.NewAttoFILFromFIL(1), 10000, abi.RegisteredProof_StackedDRG2KiBSeal, peer, types.NewAttoFILFromFIL(1))
	require.NoError(t, err)

	// inspect results on chain
	tsk := newMiner.Chain().ChainReader.GetHead()
	view, err := newMiner.Chain().ActorState.StateView(tsk)
	require.NoError(t, err)
	owner, _, err := view.MinerControlAddresses(ctx, minerAddr)
	require.NoError(t, err)

	resolvedDefaultAddress, err := view.InitResolveAddress(ctx, defaultAddr)
	require.NoError(t, err)

	assert.Equal(t, resolvedDefaultAddress, owner)
}

func TestSetPrice(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, cancel1 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel1()

	nodes, cancel2 := test.MustCreateNodesWithBootstrap(ctx, t, 0)
	defer cancel2()

	env := commands.CreateServerEnv(ctx, nodes[0])

	err := commands.GetStorageAPI(env).AddAsk(abi.NewTokenAmount(1000), abi.ChainEpoch(400))
	require.NoError(t, err)

	minerAddr, err := commands.GetBlockAPI(env).MinerAddress()
	require.NoError(t, err)

	asks, err := commands.GetStorageAPI(env).ListAsks(minerAddr)
	require.NoError(t, err)
	require.Len(t, asks, 1)
	assert.Equal(t, abi.NewTokenAmount(1000), asks[0].Ask.Price)
	assert.Equal(t, abi.ChainEpoch(400), asks[0].Ask.Expiry)
}
