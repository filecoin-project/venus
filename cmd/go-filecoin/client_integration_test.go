package commands_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"

	"github.com/filecoin-project/go-fil-markets/storagemarket"

	commands "github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestProposeDeal(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	nodes, cancel := test.MustCreateNodesWithBootstrap(ctx, t, 1)
	defer cancel()

	miner := nodes[0]

	// get miner stats
	maddr, err := miner.BlockMining.BlockMiningAPI.MinerAddress()
	require.NoError(t, err)

	mstats, err := miner.PorcelainAPI.MinerGetStatus(ctx, maddr, miner.PorcelainAPI.ChainHeadKey())
	require.NoError(t, err)

	client := nodes[1]

	clientAddr, err := client.PorcelainAPI.WalletDefaultAddress()
	require.NoError(t, err)

	// Add enough funds (1 FIL) for client and miner to to cover deal
	err = miner.StorageProtocol.Provider().AddStorageCollateral(ctx, types.NewAttoFILFromFIL(1))
	require.NoError(t, err)
	err = client.StorageProtocol.Client().AddPaymentEscrow(ctx, clientAddr, types.NewAttoFILFromFIL(1))
	require.NoError(t, err)

	// import empty 1K of bytes to create piece
	input := bytes.NewBuffer(make([]byte, 1024))
	node, err := client.PorcelainAPI.DAGImportData(ctx, input)
	require.NoError(t, err)

	// propose deal
	out, err := test.RunCommandInProcess(ctx, client, commands.ClientProposeStorageDealCmd,
		cmdkit.OptMap{
			"peerid": miner.Host().ID().String(),
		},
		mstats.ActorAddress.String(),
		node.Cid().String(),
		"1000",
		"2000",
		".0000000000001",
		"1",
	)
	require.NoError(t, err)
	res := out.(*storagemarket.ProposeStorageDealResult)

	// wait for deal to process
	for i := 0; i < 30; i++ {
		out, err := test.RunCommandInProcess(ctx, client, commands.ClientQueryStorageDealCmd, nil, res.ProposalCid.String())
		require.NoError(t, err)

		deal := out.(storagemarket.ClientDeal)
		switch deal.State {
		case storagemarket.StorageDealUnknown,
			storagemarket.StorageDealValidating:
			time.Sleep(1 * time.Second) // in progress, wait and continue
		case storagemarket.StorageDealProposalAccepted,
			storagemarket.StorageDealStaged,
			storagemarket.StorageDealSealing,
			storagemarket.StorageDealActive:

			// Deal accepted. Test passed.
			return
		default:
			t.Errorf("unexpected state: %d %s", deal.State, deal.Message)
			return
		}
	}
	t.Error("timeout waiting for deal status update")
}
