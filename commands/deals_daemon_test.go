package commands_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/ipfs/go-ipfs-files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
)

func TestDealsRedeem(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.EnvironmentOpts{})

	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	require.NoError(t, env.GenesisMiner.MiningStart(ctx))

	clientDaemon := env.RequireNewNodeWithFunds(10000)
	minerDaemon := env.RequireNewNodeWithFunds(10000)

	collateral := big.NewInt(int64(1))
	price := big.NewFloat(float64(0.001))
	expiry := big.NewInt(int64(10000))
	_, err := series.CreateStorageMinerWithAsk(ctx, minerDaemon, collateral, price, expiry)
	require.NoError(t, err)

	f := files.NewBytesFile([]byte("HODLHODLHODL"))
	dataCid, err := clientDaemon.ClientImport(ctx, f)
	require.NoError(t, err)

	minerOwnerAddresses, err := minerDaemon.AddressLs(ctx)
	require.NoError(t, err)
	minerOwnerAddress := minerOwnerAddresses[0]

	oldWalletBalance, err := minerDaemon.WalletBalance(ctx, minerOwnerAddress)
	require.NoError(t, err)

	var minerAddress address.Address
	err = minerDaemon.ConfigGet(ctx, "mining.minerAddress", &minerAddress)
	require.NoError(t, err)

	dealResponse, err := clientDaemon.ClientProposeStorageDeal(ctx, dataCid, minerAddress, 0, 1, true)
	require.NoError(t, err)

	err = series.WaitForDealState(ctx, clientDaemon, dealResponse, storagedeal.Posted)
	require.NoError(t, err)

	redeemCid, err := minerDaemon.DealsRedeem(ctx, dealResponse.ProposalCid, fast.AOPrice(big.NewFloat(0.001)), fast.AOLimit(100))
	require.NoError(t, err)

	_, err = minerDaemon.MessageWait(ctx, redeemCid)
	require.NoError(t, err)

	newWalletBalance, err := minerDaemon.WalletBalance(ctx, minerOwnerAddress)
	require.NoError(t, err)

	actualBalanceDiff := newWalletBalance.Sub(oldWalletBalance).String()
	assert.Equal(t, "1000.0119999999999998", actualBalanceDiff)
}
