package commands_test

import (
	"context"
	"crypto/rand"
	"io"
	"math/big"
	"testing"
	"time"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/specs-actors/actors/abi"
	specsbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	cid "github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	multihash "github.com/multiformats/go-multihash"

	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDealsShow(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)

	// increase block time to give it it a chance to seal
	opts := fast.FilecoinOpts{
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(100 * time.Millisecond)},
	}

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, opts)
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	clientNode := env.GenesisMiner
	minerNode := env.RequireNewNodeWithFunds(1000)

	// Connect the clientNode and the minerNode
	require.NoError(t, series.Connect(ctx, clientNode, minerNode))

	// Create a minerNode
	collateral := big.NewInt(500)           // FIL
	price := big.NewFloat(0.000000001)      // price per byte/block
	expiry := big.NewInt(24 * 60 * 60 / 30) // ~24 hours

	pparams, err := minerNode.Protocol(ctx)
	require.NoError(t, err)

	sinfo := pparams.SupportedSectors[0]

	// mine the create storage message, then mine the set ask message
	series.CtxMiningNext(ctx, 2)

	ask, err := series.CreateStorageMinerWithAsk(ctx, minerNode, collateral, price, expiry, sinfo.Size)
	require.NoError(t, err)

	// enable storage protocol
	err = minerNode.MiningSetup(ctx)
	require.NoError(t, err)

	// Create some data that is the full sector size and make it autoseal asap

	maxBytesi64 := int64(getMaxUserBytesPerStagedSector())
	dataReader := io.LimitReader(rand.Reader, maxBytesi64)

	// mine the createChannel message needed to create a storage proposal
	series.CtxMiningNext(ctx, 1)

	_, deal, err := series.ImportAndStore(ctx, clientNode, ask, files.NewReaderFile(dataReader))
	require.NoError(t, err)

	t.Run("showDeal outputs correct information", func(t *testing.T) {
		res, err := clientNode.DealsShow(ctx, deal.Proposal)
		require.NoError(t, err)

		assert.Equal(t, uint64(10), res.Duration)
		assert.Equal(t, ask.Miner, res.Miner)
		assert.Equal(t, storagemarket.StorageDealProposalAccepted, res.State)

		duri64 := int64(res.Duration)
		durXmax := specsbig.NewInt(duri64 * maxBytesi64)

		totalPrice := specsbig.Mul(ask.Price, durXmax)

		assert.True(t, totalPrice.Equals(*res.TotalPrice))
	})

	t.Run("When deal does not exist says deal not found", func(t *testing.T) {
		deal.Proposal = requireTestCID(t, []byte("anything"))
		showDeal, err := clientNode.DealsShow(ctx, deal.Proposal)
		assert.Error(t, err, "Error: deal not found")
		assert.Nil(t, showDeal)
	})
}

func getMaxUserBytesPerStagedSector() uint64 {
	return uint64(abi.PaddedPieceSize(constants.DevSectorSize).Unpadded())
}

func requireTestCID(t *testing.T, data []byte) cid.Cid {
	hash, err := multihash.Sum(data, multihash.SHA2_256, -1)
	require.NoError(t, err)
	return cid.NewCidV1(cid.DagCBOR, hash)
}
