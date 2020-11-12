package commands_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commands "github.com/filecoin-project/venus/cmd/go-filecoin"
	"github.com/filecoin-project/venus/fixtures/fortest"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin/reward"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
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
		addrs[i], err = n.PorcelainAPI.WalletNewAddress(address.SECP256K1)
		require.NoError(t, err)
	}

	list := cmdClient.RunSuccess(ctx, "address", "ls").ReadStdout()
	for _, addr := range addrs {
		assert.Contains(t, list, addr.String())
	}
}

func TestWalletBalance(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	builder := test.NewNodeBuilder(t)
	cs := node.FixtureChainSeed(t)
	builder.WithGenesisInit(cs.GenesisInitFunc)

	n, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()
	addr, err := n.PorcelainAPI.WalletNewAddress(address.SECP256K1)
	require.NoError(t, err)

	t.Log("[success] not found, zero")
	var balance abi.TokenAmount
	cmdClient.RunMarshaledJSON(ctx, &balance, "wallet", "balance", addr.String())
	assert.Equal(t, "0", balance.String())

	t.Log("[success] balance 1394000000000000000000000000")
	cmdClient.RunMarshaledJSON(ctx, &balance, "wallet", "balance", reward.Address.String())
	assert.Equal(t, "1394000000000000000000000000", balance.String())

	t.Log("[success] newly generated one")
	var addrNew commands.AddressResult
	cmdClient.RunMarshaledJSON(ctx, &addrNew, "address", "new")
	cmdClient.RunMarshaledJSON(ctx, &balance, "wallet", "balance", addrNew.Address.String())
	assert.Equal(t, "0", balance.String())
}

func TestWalletLoadFromFile(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	builder := test.NewNodeBuilder(t)
	cs := node.FixtureChainSeed(t)
	builder.WithGenesisInit(cs.GenesisInitFunc)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	for _, p := range fortest.KeyFilePaths() {
		cmdClient.RunSuccess(ctx, "wallet", "import", p)
	}

	var addrs commands.AddressLsResult
	cmdClient.RunMarshaledJSON(ctx, &addrs, "address", "ls")

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

	var lsResult commands.AddressLsResult
	cmdClient.RunMarshaledJSON(ctx, &lsResult, "address", "ls")
	require.Len(t, lsResult.Addresses, 1)

	exportJSON := cmdClient.RunSuccess(ctx, "wallet", "export", lsResult.Addresses[0].String()).ReadStdout()
	var exportResult commands.WalletSerializeResult
	err := json.Unmarshal([]byte(exportJSON), &exportResult)
	require.NoError(t, err)

	wf, err := os.Create("walletFileTest")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Remove("walletFileTest"))
	}()

	_, err = wf.WriteString(exportJSON)
	require.NoError(t, err)
	require.NoError(t, wf.Close())

	var importResult commands.AddressLsResult
	cmdClient.RunMarshaledJSON(ctx, &importResult, "wallet", "import", wf.Name())
	assert.Len(t, importResult.Addresses, 1)
	assert.Equal(t, lsResult.Addresses[0], importResult.Addresses[0])
}
