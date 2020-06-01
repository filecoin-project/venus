package commands_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/multiformats/go-multihash"

	"github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	commands "github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestDealsList(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	minerAPI, clientAPI, done, deal1Cid, deal2Cid := createDeal(t, ctx)
	defer done()

	var dealResults []commands.DealsListResult
	var cidsInList [2]cid.Cid
	t.Run("with no filters", func(t *testing.T) {
		// Client fails cause no miner is started for the client
		clientAPI.RunFail(ctx, "Error: error reading miner deals: Mining has not been started so storage provider is not available", "deals", "list")

		// Miner sees the deal
		minerAPI.RunMarshaledJSON(ctx, &dealResults, "deals", "list")
		require.Len(t, dealResults, 2)
		cidsInList[0] = dealResults[0].ProposalCid
		cidsInList[1] = dealResults[1].ProposalCid
		require.Contains(t, cidsInList, deal1Cid)
		require.Contains(t, cidsInList, deal2Cid)
	})

	t.Run("with --miner", func(t *testing.T) {
		// Client fails cause no miner is started for the client
		clientAPI.RunFail(ctx, "Error: error reading miner deals: Mining has not been started so storage provider is not available", "deals", "list")

		// Miner sees the deal
		minerAPI.RunMarshaledJSON(ctx, &dealResults, "deals", "list", "--miner")
		require.Len(t, dealResults, 2)
		cidsInList[0] = dealResults[0].ProposalCid
		cidsInList[1] = dealResults[1].ProposalCid
		require.Contains(t, cidsInList, deal1Cid)
		require.Contains(t, cidsInList, deal2Cid)
	})

	t.Run("with --client", func(t *testing.T) {
		// Client sees both deals
		clientAPI.RunMarshaledJSON(ctx, &dealResults, "deals", "list", "--client")
		require.Len(t, dealResults, 2)
		cidsInList[0] = dealResults[0].ProposalCid
		cidsInList[1] = dealResults[1].ProposalCid
		require.Contains(t, cidsInList, deal1Cid)
		require.Contains(t, cidsInList, deal2Cid)

		// Miner sees no client deals, but does not error
		minerOutput := minerAPI.RunSuccessFirstLine(ctx, "deals", "list", "--client")
		require.Equal(t, "[]", minerOutput)
	})

	t.Run("with --help", func(t *testing.T) {
		clientOutput := clientAPI.RunSuccess(ctx, "deals", "list", "--help").ReadStdoutTrimNewlines()
		require.Contains(t, clientOutput, "only return deals made as a client")
		require.Contains(t, clientOutput, "only return deals made as a miner")
	})
}

func TestDealShow(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	_, clientAPI, done, deal1Cid, _ := createDeal(t, ctx)
	defer done()

	var res storagemarket.ClientDeal
	t.Run("showDeal outputs correct information", func(t *testing.T) {
		clientAPI.RunMarshaledJSON(ctx, &res, "deals", "show", deal1Cid.String())

		assert.Equal(t, abi.ChainEpoch(2000), res.ClientDealProposal.Proposal.EndEpoch)
		assert.Equal(t, "t0106", res.ClientDealProposal.Proposal.Provider.String())
		assert.LessOrEqual(t, storagemarket.StorageDealProposalAccepted, res.State)

		assert.Equal(t, abi.NewTokenAmount(100000), res.ClientDealProposal.Proposal.StoragePricePerEpoch)
	})

	t.Run("When deal does not exist says deal not found", func(t *testing.T) {
		nonDealCid := requireTestCID(t, []byte("anything"))
		clientAPI.RunFail(ctx, "deal not found", "deals", "show", nonDealCid.String())
	})
}

func createDeal(t *testing.T, ctx context.Context) (*test.Client, *test.Client, func(), cid.Cid, cid.Cid) {
	nodes, cancel := test.MustCreateNodesWithBootstrap(ctx, t, 1)

	miner := nodes[0]
	minerAPI, minerStop := test.RunNodeAPI(ctx, miner, t)

	maddr, err := miner.BlockMining.BlockMiningAPI.MinerAddress()
	require.NoError(t, err)

	client := nodes[1]
	clientAPI, clientStop := test.RunNodeAPI(ctx, client, t)
	done := func() {
		cancel()
		minerStop()
		clientStop()
	}

	clientAddr, err := client.PorcelainAPI.WalletDefaultAddress()
	require.NoError(t, err)

	// Add enough funds (1 FIL) for client and miner to to cover deal
	provider, err := miner.StorageProtocol.Provider()
	require.NoError(t, err)

	err = provider.AddStorageCollateral(ctx, types.NewAttoFILFromFIL(1))
	require.NoError(t, err)
	err = client.StorageProtocol.Client().AddPaymentEscrow(ctx, clientAddr, types.NewAttoFILFromFIL(1))
	require.NoError(t, err)

	// import some data to create first piece
	input1 := bytes.NewBuffer([]byte("HODLHODLHODL"))
	node1, err := client.PorcelainAPI.DAGImportData(ctx, input1)
	require.NoError(t, err)

	// import some data to create second piece
	input2 := bytes.NewBuffer([]byte("FREEASINBEER"))
	node2, err := client.PorcelainAPI.DAGImportData(ctx, input2)
	require.NoError(t, err)

	// propose 2 deals
	var result storagemarket.ProposeStorageDealResult
	clientAPI.RunMarshaledJSON(ctx, &result, "client", "propose-storage-deal",
		"--peerid", miner.Host().ID().String(),
		maddr.String(),
		node1.Cid().String(),
		"1000",
		"2000",
		".0000000000001",
		"1",
	)
	require.NotEqual(t, cid.Undef, result.ProposalCid)
	deal1Cid := result.ProposalCid

	clientAPI.RunMarshaledJSON(ctx, &result, "client", "propose-storage-deal",
		"--peerid", miner.Host().ID().String(),
		maddr.String(),
		node2.Cid().String(),
		"1000",
		"2000",
		".0000000000001",
		"1",
	)
	require.NotEqual(t, cid.Undef, result.ProposalCid)
	deal2Cid := result.ProposalCid
	return minerAPI, clientAPI, done, deal1Cid, deal2Cid
}

func requireTestCID(t *testing.T, data []byte) cid.Cid {
	hash, err := multihash.Sum(data, multihash.SHA2_256, -1)
	require.NoError(t, err)
	return cid.NewCidV1(cid.DagCBOR, hash)
}
