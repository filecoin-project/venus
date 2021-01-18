package cmd_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/filecoin-project/venus/pkg/crypto"
	"os"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/venus/app/node/test"
	"github.com/filecoin-project/venus/cmd"
	"github.com/filecoin-project/venus/fixtures/fortest"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestAddressNewAndList(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	n, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	addrs := make([]address.Address, 10)
	var err error
	for i := 0; i < 10; i++ {
		addrs[i], err = n.Wallet().API().WalletNewAddress(address.SECP256K1)
		require.NoError(t, err)
	}

	list := cmdClient.RunSuccess(ctx, "wallet", "ls").ReadStdout()
	for _, addr := range addrs {
		assert.Contains(t, list, addr.String())
	}
}

func TestWalletBalance(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	builder := test.NewNodeBuilder(t)
	cs := test.FixtureChainSeed(t)
	builder.WithGenesisInit(cs.GenesisInitFunc)

	n, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()
	addr, err := n.Wallet().API().WalletNewAddress(address.SECP256K1)
	require.NoError(t, err)

	t.Log("[success] not found, zero")
	var balance abi.TokenAmount
	cmdClient.RunMarshaledJSON(ctx, &balance, "wallet", "balance", addr.String())
	assert.Equal(t, "0", balance.String())

	t.Log("[success] balance 1394000000000000000000000000")
	cmdClient.RunMarshaledJSON(ctx, &balance, "wallet", "balance", builtin.RewardActorAddr.String())
	assert.Equal(t, "1394000000000000000000000000", balance.String())

	t.Log("[success] newly generated one")
	var addrNew cmd.AddressResult
	cmdClient.RunMarshaledJSON(ctx, &addrNew, "wallet", "new")
	cmdClient.RunMarshaledJSON(ctx, &balance, "wallet", "balance", addrNew.Address.String())
	assert.Equal(t, "0", balance.String())
}

func TestWalletLoadFromFile(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	builder := test.NewNodeBuilder(t)
	cs := test.FixtureChainSeed(t)
	builder.WithGenesisInit(cs.GenesisInitFunc)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	for _, p := range fortest.KeyFilePaths() {
		cmdClient.RunSuccess(ctx, "wallet", "import", p)
	}

	var addrs cmd.AddressLsResult
	cmdClient.RunMarshaledJSON(ctx, &addrs, "wallet", "ls")

	for _, addr := range fortest.TestAddresses {
		// assert we loaded the test address from the file
		assert.Contains(t, addrs.Addresses, addr)
	}

	// assert default amount of funds were allocated to address during genesis
	var balance abi.TokenAmount
	cmdClient.RunMarshaledJSON(ctx, &balance, "wallet", "balance", fortest.TestAddresses[0].String())
	assert.Equal(t, "1000000000000000000000000", balance.String())
}

func TestWalletExportImportRoundTrip(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	var lsResult cmd.AddressLsResult
	cmdClient.RunMarshaledJSON(ctx, &lsResult, "wallet", "ls")
	require.Len(t, lsResult.Addresses, 1)

	exportJSON := cmdClient.RunSuccess(ctx, "wallet", "export", lsResult.Addresses[0].String()).ReadStdout()
	data, err := hex.DecodeString(exportJSON)
	require.NoError(t, err)
	var exportResult crypto.KeyInfo
	err = json.Unmarshal(data, &exportResult)
	require.NoError(t, err)

	wf, err := os.Create("walletFileTest")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Remove("walletFileTest"))
	}()

	keyInfo, err := json.Marshal(exportResult)
	require.NoError(t, err)
	_, err = wf.WriteString(hex.EncodeToString(keyInfo))
	require.NoError(t, err)
	require.NoError(t, wf.Close())

	var importResult address.Address
	cmdClient.RunMarshaledJSON(ctx, &importResult, "wallet", "import", wf.Name())
	assert.Equal(t, lsResult.Addresses[0], importResult)
}
