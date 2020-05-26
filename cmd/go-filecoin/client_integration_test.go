package commands_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestProposeDeal(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	nodes, cancel := test.MustCreateNodesWithBootstrap(ctx, t, 1)
	defer cancel()

	miner := nodes[0]

	maddr, err := miner.BlockMining.BlockMiningAPI.MinerAddress()
	require.NoError(t, err)

	client := nodes[1]
	clientAPI, clientStop := test.RunNodeAPI(ctx, client, t)
	defer clientStop()

	clientAddr, err := client.PorcelainAPI.WalletDefaultAddress()
	require.NoError(t, err)

	// Add enough funds (1 FIL) for client and miner to to cover deal
	provider, err := miner.StorageProtocol.Provider()
	require.NoError(t, err)

	err = provider.AddStorageCollateral(ctx, types.NewAttoFILFromFIL(1))
	require.NoError(t, err)
	err = client.StorageProtocol.Client().AddPaymentEscrow(ctx, clientAddr, types.NewAttoFILFromFIL(1))
	require.NoError(t, err)

	// import empty 1K of bytes to create piece
	input := bytes.NewBuffer(make([]byte, 1024))
	node, err := client.PorcelainAPI.DAGImportData(ctx, input)
	require.NoError(t, err)

	// propose deal
	var result storagemarket.ProposeStorageDealResult
	clientAPI.RunMarshaledJSON(ctx, &result, "client", "propose-storage-deal",
		"--peerid", miner.Host().ID().String(),
		maddr.String(),
		node.Cid().String(),
		"1000",
		"2000",
		".0000000000001",
		"1",
	)

	// wait for deal to process
	var dealStatus storagemarket.ClientDeal
	for i := 0; i < 30; i++ {
		clientAPI.RunMarshaledJSON(ctx, &dealStatus, "client", "query-storage-deal", result.ProposalCid.String())
		switch dealStatus.State {
		case storagemarket.StorageDealProposalAccepted,
			storagemarket.StorageDealStaged,
			storagemarket.StorageDealSealing,
			storagemarket.StorageDealActive:
			// Deal accepted. Test passed.
			return
		default:
			time.Sleep(1 * time.Second) // in progress, wait and continue
		}
	}
	t.Error("timeout waiting for deal status update")
}
