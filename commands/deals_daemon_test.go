package commands_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/ipfs/go-ipfs-files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestDealsRedeem(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.EnvironmentOpts{})

	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	clientDaemon := env.RequireNewNodeWithFunds(10000)
	minerDaemon := env.RequireNewNodeWithFunds(10000)

	require.NoError(t, env.GenesisMiner.MiningStart(ctx))

	collateral := big.NewInt(int64(1))
	price := big.NewFloat(float64(20))
	expiry := big.NewInt(int64(100))
	_, err := series.CreateStorageMinerWithAsk(ctx, minerDaemon, collateral, price, expiry)
	require.NoError(t, err)

	f := files.NewBytesFile([]byte("HODLHODLHODL"))
	dataCid, err := clientDaemon.ClientImport(ctx, f)
	require.NoError(t, err)

	var minerAddress address.Address
	err = minerDaemon.ConfigGet(ctx, "mining.minerAddress", &minerAddress)
	require.NoError(t, err)

	proposeDealOutput, err := clientDaemon.ClientProposeStorageDeal(ctx, dataCid, minerAddress, 0, 5, true)
	require.NoError(t, err)

	oldWalletBalance, err := minerDaemon.WalletBalance(ctx, minerAddress)
	require.NoError(t, err)

	redeemCid, err := minerDaemon.DealsRedeem(ctx, proposeDealOutput.ProposalCid, fast.AOPrice(big.NewFloat(0.001)), fast.AOLimit(100))
	require.NoError(t, err)

	_, err = clientDaemon.MessageWait(ctx, redeemCid)
	require.NoError(t, err)

	newWalletBalance, err := minerDaemon.WalletBalance(ctx, minerAddress)
	require.NoError(t, err)

	expectedBalanceDiff := types.NewAttoFILFromFIL(0).String()
	actualBalanceDiff := newWalletBalance.Sub(oldWalletBalance).String()
	assert.Equal(t, expectedBalanceDiff, actualBalanceDiff)
}
