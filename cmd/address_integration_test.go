package cmd_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/venus/app/node/test"
	"github.com/filecoin-project/venus/cmd"
	"github.com/filecoin-project/venus/fixtures/fortest"
	"github.com/filecoin-project/venus/pkg/crypto"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/wallet"
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
		addrs[i], err = n.Wallet().API().WalletNewAddress(ctx, address.SECP256K1)
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

	addr, err := n.Wallet().API().WalletNewAddress(ctx, address.SECP256K1)
	require.NoError(t, err)

	t.Log("[success] not found, zero")
	balance := cmdClient.RunSuccess(ctx, "wallet", "balance", addr.String()).ReadStdout()
	assert.Equal(t, "0 FIL\n", balance)

	t.Log("[success] balance 1394000000000000000000000000")
	balance = cmdClient.RunSuccess(ctx, "wallet", "balance", builtin.RewardActorAddr.String()).ReadStdout()
	assert.Equal(t, "1394000000 FIL\n", balance)

	t.Log("[success] newly generated one")
	var addrNew cmd.AddressResult
	cmdClient.RunSuccessFirstLine(ctx, "wallet", "new")
	balance = cmdClient.RunSuccess(ctx, "wallet", "balance", addrNew.Address.String()).ReadStdout()
	assert.Equal(t, "0 FIL\n", balance)
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

	list := cmdClient.RunSuccess(ctx, "wallet", "ls").ReadStdout()
	for _, addr := range fortest.TestAddresses {
		// assert we loaded the test address from the file
		assert.Contains(t, list, addr.String())
	}

	// assert default amount of funds were allocated to address during genesis
	balance := cmdClient.RunSuccess(ctx, "wallet", "balance", fortest.TestAddresses[0].String()).ReadStdout()
	assert.Equal(t, "1000000 FIL\n", balance)
}

func TestWalletExportImportRoundTrip(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	n, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	addr, err := n.Wallet().API().WalletNewAddress(ctx, address.SECP256K1)
	require.NoError(t, err)

	// ./venus wallet ls
	// eg:
	// Address                                                                                 Balance  Nonce  Default
	// t3wzm53n4ui4zdgwenf7jflrtsejgpsus7rswlkvbffxhdpkixpzfzidbvinrpnjx7dgvs72ilsnpiu7yjhela  0 FIL    0      X
	result := cmdClient.RunSuccessLines(ctx, "wallet", "ls")
	require.Len(t, result, 2) // include the header `Address Balance  Nonce  Default`
	resultAddr := strings.Split(result[1], " ")[0]
	require.Equal(t, addr.String(), resultAddr)

	exportJSON := cmdClient.RunSuccess(ctx, "wallet", "export", resultAddr, string(wallet.TestPassword)).ReadStdoutTrimNewlines()
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

	keyInfoByte, err := json.Marshal(exportResult)
	require.NoError(t, err)
	_, err = wf.WriteString(hex.EncodeToString(keyInfoByte))
	require.NoError(t, err)
	require.NoError(t, wf.Close())

	importResult := cmdClient.RunSuccessFirstLine(ctx, "wallet", "import", wf.Name())
	assert.Equal(t, resultAddr, importResult)
}
