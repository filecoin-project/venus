package commands_test

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/ipfs/go-ipfs-files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
)

func TestSelfDialRetrievalGoodError(t *testing.T) {
	tf.IntegrationTest(t)

	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()

	ctx, env := fastesting.NewTestEnvironment(ctx, t, fast.EnvironmentOpts{})
	// Teardown after test ends.
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(err)
	}()

	// Update genesis miner's peerid
	var minerAddr address.Address
	err := env.GenesisMiner.ConfigGet(ctx, "mining.minerAddress", &minerAddr)
	require.NoError(err)
	details, err := env.GenesisMiner.ID(ctx)
	require.NoError(err)
	msgCid, err := env.GenesisMiner.MinerUpdatePeerid(ctx, minerAddr, details.ID, fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	require.NoError(err)

	series.CtxMiningOnce(ctx)
	_, err = env.GenesisMiner.MessageWait(ctx, msgCid)
	require.NoError(err)

	// Add data to Genesis Miner.
	f := files.NewBytesFile([]byte("satyamevajayate"))
	cid, err := env.GenesisMiner.ClientImport(ctx, f)
	require.NoError(err)

	// Genesis Miner fails on self dial when retrieving from itself.
	_, err = env.GenesisMiner.RetrievalClientRetrievePiece(ctx, cid, minerAddr)
	assert.Error(err)
	var cmdOutBytes []byte
	w := bytes.NewBuffer(cmdOutBytes)
	env.GenesisMiner.DumpLastOutput(w)
	outputStr := string(w.Bytes())
	expectedErrStr := "attempting to retrieve piece from self"
	assert.Contains(outputStr, expectedErrStr)
}
