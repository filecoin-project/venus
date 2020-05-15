package commands_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/pieceio"
	"github.com/filecoin-project/go-fil-markets/pieceio/cario"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	testing2 "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

func TestRetrieveFromSelf(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	nodes, canc1 := test.MustCreateNodesWithBootstrap(ctx, t, 1)
	defer canc1()

	newMiner := nodes[1]

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

	// minerAddr won't be used since we'll get the data locally but right now
	// it's required
	minerAddr := testing2.NewIDAddr(t, 1001)
	testout := cmdClient.RunSuccess(ctx, "retrieval-client", "retrieve-piece",
		minerAddr.String(), out.Cid().String())

	assert.Equal(t, contents, testout.Stdout())
}

func TestRetrieveFromPeer(t *testing.T) {
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

	payloadCID := makeOfflineStorageDeal(ctx, t, retMiner)

	cmdClient, apiDone := test.RunNodeAPI(ctx, retClient, t)
	defer func() {
		apiDone()
		retClient.Stop(ctx)
	}()
	testout := cmdClient.RunSuccess(ctx, "retrieval-client", "retrieve-piece",
		minerAddr.String(), payloadCID.String())

	assert.NotEmpty(t, testout.Stdout())
	assert.Empty(t, testout.Stderr())
}

func TestErrorConditions(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	testCases := []testParams{
		{name: "cannot make 0 size deal", content: string([]byte{}), expErr: "cannot make retrieval deal for 0 bytes"},
		//{name: "peer is not found", miner: testing2.NewIDAddr(t, 1234).String(), expErr: "bar"},
		//{name: "insufficient balance in wallet", expErr: "bazz"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runErrorTest(ctx, t, tc)
		})
	}
}

type testParams struct {
	name, payloadCID, miner, content, expErr string
	balance                                  uint64
}

func runErrorTest(ctx context.Context, t *testing.T, params testParams) {
	newParams := params

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

	// create a file to import, retrieval miner imports the data
	contents := []byte(newParams.content)
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

	if newParams.payloadCID == "" {
		newParams.payloadCID = out.Cid().String()
	}
	if newParams.miner == "" {
		newParams.miner = minerAddr.String()
	}
	if newParams.balance > 0 {
		adjustWalletBalanceTo(ctx, t, retClient, nodes[0], newParams.balance)
	}

	testout := cmdClient.RunSuccess(ctx, "retrieval-client", "retrieve-piece",
		newParams.miner, newParams.payloadCID)

	assert.Equal(t, []byte(newParams.expErr), testout.Stderr())

}

func adjustWalletBalanceTo(ctx context.Context, t *testing.T, from, to *node.Node, newBal uint64) {
	fromAddr, err := from.PorcelainAPI.WalletDefaultAddress()
	require.NoError(t, err)
	toAddr, err := to.PorcelainAPI.WalletDefaultAddress()
	require.NoError(t, err)
	currentBal, err := from.PorcelainAPI.WalletBalance(ctx, fromAddr)
	require.NoError(t, err)
	sendAmtBig := currentBal.Sub(currentBal.Int, types.NewAttoFILFromFIL(newBal).Int)
	sendAmt := types.NewAttoFIL(sendAmtBig)
	gasPrice := types.NewAttoFILFromFIL(1)
	gasLimit := gas.NewGas(5000)

	msgCID, _, err := from.PorcelainAPI.MessageSend(ctx, fromAddr, toAddr, sendAmt, gasPrice, gasLimit, builtin.MethodSend, nil)
	require.NoError(t, err)
	toctx, canc := context.WithTimeout(ctx, 30*time.Second)
	defer canc()
	rcpt, err := from.PorcelainAPI.MessageWaitDone(toctx, msgCID)
	require.NoError(t, err)
	require.Equal(t, exitcode.Ok, rcpt.ExitCode)
}

func makeOfflineStorageDeal(ctx context.Context, t *testing.T, miner *node.Node) cid.Cid {
	fpath := filepath.Join("storagemarket", "fixtures", "payload.txt")

	testData := shared_testutil.NewLibp2pTestData(ctx, t)
	rootLink := testData.LoadUnixFSFile(t, fpath, false)
	payloadCID := rootLink.(cidlink.Link).Cid

	carBuf := new(bytes.Buffer)

	err := cario.NewCarIO().WriteCar(ctx, testData.Bs1, payloadCID, shared.AllSelector(), carBuf)
	require.NoError(t, err)

	commP, size, err := pieceio.GeneratePieceCommitment(abi.RegisteredProof_StackedDRG2KiBPoSt, carBuf, uint64(carBuf.Len()))
	assert.NoError(t, err)

	dataRef := &storagemarket.DataRef{
		TransferType: storagemarket.TTManual,
		Root:         payloadCID,
		PieceCid:     &commP,
		PieceSize:    size,
	}

	res, err := miner.PorcelainAPI.ConfigGet("mining.minerAddress")
	require.NoError(t, err)
	providerAddr, ok := res.(address.Address)
	require.True(t, ok)
	providerInfo := storagemarket.StorageProviderInfo{
		Address:    providerAddr,
		Owner:      providerAddr,
		Worker:     providerAddr,
		SectorSize: 1 << 20,
		PeerID:     testData.Host1.ID(),
	}
	result, err := miner.StorageAPI.ProposeStorageDeal(
		ctx,
		providerAddr,
		&providerInfo,
		dataRef,
		100,
		20100,
		big.NewInt(1),
		big.NewInt(0),
		abi.RegisteredProof_StackedDRG2KiBPoSt,
	)
	require.NoError(t, err)
	proposalCid := result.ProposalCid

	time.Sleep(time.Millisecond * 100)

	cd, err := miner.StorageProtocol.Client().GetLocalDeal(ctx, proposalCid)
	assert.NoError(t, err)
	assert.Equal(t, storagemarket.StorageDealValidating, cd.State)

	providerDeals, err := miner.StorageProtocol.StorageProvider.ListLocalDeals()
	assert.NoError(t, err)

	pd := providerDeals[0]
	assert.True(t, pd.ProposalCid.Equals(proposalCid))
	assert.Equal(t, storagemarket.StorageDealWaitingForData, pd.State)

	err = cario.NewCarIO().WriteCar(ctx, testData.Bs1, payloadCID, shared.AllSelector(), carBuf)
	require.NoError(t, err)
	err = miner.StorageProtocol.StorageProvider.ImportDataForDeal(ctx, pd.ProposalCid, carBuf)
	require.NoError(t, err)
	return payloadCID
}
