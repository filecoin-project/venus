package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/specs-actors/actors/abi"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestRetrieveFromSelf(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	nodes, canc1 := test.MustCreateNodesWithBootstrap(ctx, t, 1)
	defer canc1()

	newMiner := nodes[1]

	wallet, err := newMiner.PorcelainAPI.WalletDefaultAddress()
	require.NoError(t, err)

	toctx, canc2 := context.WithTimeout(ctx, 30*time.Second)
	defer canc2()
	peer := newMiner.PorcelainAPI.NetworkGetPeerID()
	minerAddr, err := newMiner.PorcelainAPI.MinerCreate(toctx, wallet, types.NewAttoFILFromFIL(1), 10000, abi.RegisteredProof_StackedDRG2KiBSeal, peer, types.NewAttoFILFromFIL(1))
	require.NoError(t, err)

	configMinerAddr, err := newMiner.PorcelainAPI.ConfigGet("mining.minerAddress")
	require.NoError(t, err)
	require.Equal(t, minerAddr, configMinerAddr)

	// setup mining so retrieval protocol gets set up
	require.NoError(t, newMiner.SetupMining(ctx))

	// create a file to import
	contents := []byte("satyamevajayate")
	f := files.NewBytesFile(contents)
	out, err := newMiner.PorcelainAPI.DAGImportData(ctx, f)
	require.NoError(t, err)

	nodeStat, err := out.Stat()
	require.NoError(t, err)
	require.NotZero(t, nodeStat.DataSize)

	cmdClient, apiDone := test.RunNodeAPI(ctx, newMiner, t)
	defer func() {
		apiDone()
		newMiner.Stop(ctx)
	}()
	testout := cmdClient.RunSuccess(ctx, "retrieval-client", "retrieve-piece",
		minerAddr.String(), out.Cid().String())

	assert.Equal(t, contents, testout.Stdout())
}

func TestRetrieveFromOther(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	nodes, canc1 := test.MustCreateNodesWithBootstrap(ctx, t, 2)
	defer canc1()

	retMiner := nodes[1]
	retClient := nodes[2]

	wallet, err := retMiner.PorcelainAPI.WalletDefaultAddress()
	require.NoError(t, err)

	toctx2, canc2 := context.WithTimeout(ctx, 30*time.Second)
	defer canc2()
	peer := retMiner.PorcelainAPI.NetworkGetPeerID()
	minerAddr, err := retMiner.PorcelainAPI.MinerCreate(toctx2, wallet, types.NewAttoFILFromFIL(1), 10000, abi.RegisteredProof_StackedDRG2KiBSeal, peer, types.NewAttoFILFromFIL(1))
	require.NoError(t, err)
	require.NoError(t, retMiner.SetupMining(ctx))

	toctx3, canc3 := context.WithTimeout(ctx, 30*time.Second)
	defer canc3()
	clientPeer := retClient.PorcelainAPI.NetworkGetPeerID()
	clientWallet, err := retClient.PorcelainAPI.WalletDefaultAddress()
	require.NoError(t, err)
	_, err = retClient.PorcelainAPI.MinerCreate(toctx3, clientWallet, types.NewAttoFILFromFIL(1), 10000, abi.RegisteredProof_StackedDRG2KiBSeal, clientPeer, types.NewAttoFILFromFIL(1))
	require.NoError(t, err)
	require.NoError(t, retClient.SetupMining(ctx))

	// create a file to import
	contents := []byte("satyamevajayate")
	f := files.NewBytesFile(contents)
	out, err := retMiner.PorcelainAPI.DAGImportData(ctx, f)
	require.NoError(t, err)

	_, err = out.Stat()
	require.NoError(t, err)

	cmdClient, apiDone := test.RunNodeAPI(ctx, retClient, t)
	defer func() {
		apiDone()
		retClient.Stop(ctx)
	}()
	testout := cmdClient.RunSuccess(ctx, "retrieval-client", "retrieve-piece",
		minerAddr.String(), out.Cid().String())

	assert.Equal(t, contents, testout.Stdout())

}
