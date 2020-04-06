package commands_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	commands "github.com/filecoin-project/go-filecoin/cmd/go-filecoin"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestAddrsNewAndList(t *testing.T) {
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
	t.Skip("not working")
	ctx := context.Background()

	builder := test.NewNodeBuilder(t)
	cs := node.FixtureChainSeed(t)
	builder.WithGenesisInit(cs.GenesisInitFunc)

	n, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()
	addr, err := n.PorcelainAPI.WalletNewAddress(address.SECP256K1)
	require.NoError(t, err)

	t.Log("[success] not found, zero")
	balance := cmdClient.RunSuccess(ctx, "wallet", "balance", addr.String())
	assert.Equal(t, "0", balance.ReadStdoutTrimNewlines())

	t.Log("[success] balance 9999900000")
	balance = cmdClient.RunSuccess(ctx, "wallet", "balance", builtin.RewardActorAddr.String())
	assert.Equal(t, "949999900000", balance.ReadStdoutTrimNewlines())

	t.Log("[success] newly generated one")
	addrNew := cmdClient.RunSuccess(ctx, "address", "new")
	balance = cmdClient.RunSuccess(ctx, "wallet", "balance", addrNew.ReadStdoutTrimNewlines())
	assert.Equal(t, "0", balance.ReadStdoutTrimNewlines())
}

func TestAddrLookupAndUpdate(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)
	ctx := context.Background()

	builder := test.NewNodeBuilder(t)
	cs := node.FixtureChainSeed(t)

	builder.WithGenesisInit(cs.GenesisInitFunc)
	n1, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	builder2 := test.NewNodeBuilder(t)
	builder2.WithConfig(cs.MinerConfigOpt(0))
	builder2.WithInitOpt(cs.KeyInitOpt(0))
	builder2.WithInitOpt(cs.KeyInitOpt(1))

	n2 := builder2.BuildAndStart(ctx)
	defer n2.Stop(ctx)

	node.ConnectNodes(t, n1, n2)

	addr := fixtures.TestAddresses[0]
	minerAddr := fixtures.TestMiners[0]
	minerPidForUpdate := th.RequireRandomPeerID(t)

	// capture original, pre-update miner pid
	lookupOutA := cmdClient.RunSuccessFirstLine(ctx, "address", "lookup", minerAddr.String())

	// Not a miner address, should fail.
	cmdClient.RunFail(ctx, "failed to find", "address", "lookup", addr.String())

	// update the miner's peer ID
	updateMsg := cmdClient.RunSuccessFirstLine(ctx,
		"miner", "update-peerid",
		"--from", addr.String(),
		"--gas-price", "1",
		"--gas-limit", "300",
		minerAddr.String(),
		minerPidForUpdate.Pretty(),
	)

	// ensure mining happens after update message gets included in mempool
	_, err := n2.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	// wait for message to be included in a block
	_, err = n1.PorcelainAPI.MessageWaitDone(ctx, mustDecodeCid(updateMsg))
	require.NoError(t, err)

	// use the address lookup command to ensure update happened
	lookupOutB := cmdClient.RunSuccessFirstLine(ctx, "address", "lookup", minerAddr.String())
	assert.Equal(t, minerPidForUpdate.Pretty(), lookupOutB)
	assert.NotEqual(t, lookupOutA, lookupOutB)
}

func TestWalletLoadFromFile(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("not working")
	ctx := context.Background()

	builder := test.NewNodeBuilder(t)

	buildWithMiner(t, builder)
	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	for _, p := range fixtures.KeyFilePaths() {
		cmdClient.RunSuccess(ctx, "wallet", "import", p)
	}

	dw := cmdClient.RunSuccess(ctx, "address", "ls").ReadStdoutTrimNewlines()

	for _, addr := range fixtures.TestAddresses {
		// assert we loaded the test address from the file
		assert.Contains(t, dw, addr)
	}

	// assert default amount of funds were allocated to address during genesis
	wb := cmdClient.RunSuccess(ctx, "wallet", "balance", fixtures.TestAddresses[0].String()).ReadStdoutTrimNewlines()
	assert.Contains(t, wb, "10000")
}

func TestWalletExportImportRoundTrip(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	lsJSON := cmdClient.RunSuccess(ctx, "address", "ls").ReadStdoutTrimNewlines()
	var lsResult commands.AddressLsResult
	err := json.Unmarshal([]byte(lsJSON), &lsResult)
	require.NoError(t, err)
	require.Len(t, lsResult.Addresses, 1)

	exportJSON := cmdClient.RunSuccess(ctx, "wallet", "export", lsResult.Addresses[0].String(), "--enc=json").ReadStdout()
	var exportResult commands.WalletSerializeResult
	err = json.Unmarshal([]byte(exportJSON), &exportResult)
	require.NoError(t, err)

	wf, err := os.Create("walletFileTest")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Remove("walletFileTest"))
	}()

	_, err = wf.WriteString(exportJSON)
	require.NoError(t, err)
	require.NoError(t, wf.Close())

	maybeAddr := cmdClient.RunSuccess(ctx, "wallet", "import", wf.Name()).ReadStdoutTrimNewlines()
	assert.Equal(t, lsJSON, maybeAddr)
}

// MustDecodeCid decodes a string to a Cid pointer, panicking on error
func mustDecodeCid(cidStr string) cid.Cid {
	decode, err := cid.Decode(cidStr)
	if err != nil {
		panic(err)
	}

	return decode
}
