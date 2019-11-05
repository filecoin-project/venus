package networkdeployment_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"math/big"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs-files"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	minerActor "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
)

func TestMinerWorkflow(t *testing.T) {
	network := tf.DeploymentTest(t)

	ctx := context.Background()
	ctx, env := fastesting.NewDeploymentEnvironment(ctx, t, network, fast.FilecoinOpts{})
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(t, err)
	}()

	miner := env.RequireNewNodeWithFunds()
	client := env.RequireNewNodeWithFunds()

	t.Run("Verify mining", func(t *testing.T) {
		askCollateral := big.NewInt(10)
		askPrice := big.NewFloat(0.000000001)
		askExpiry := big.NewInt(128)
		dealExpiry := uint64(32)

		pparams, err := miner.Protocol(ctx)
		require.NoError(t, err)

		sinfo := pparams.SupportedSectors[0]

		//
		// Verify that a miner can be created with an ask
		//

		t.Logf("Sector Size: %s", sinfo.Size)

		// Create a miner on the miner node
		ask, err := series.CreateStorageMinerWithAsk(ctx, miner, askCollateral, askPrice, askExpiry, sinfo.Size)
		require.NoError(t, err)

		t.Logf("Miner  %s", ask.Miner)
		t.Logf("Ask ID %d", ask.ID)

		//
		// Verify that a deal can be proposed
		//

		// Connect the client and the miner
		err = series.Connect(ctx, client, miner)
		require.NoError(t, err)

		// Store some data with the miner with the given ask, returns the cid for
		// the imported data, and the deal which was created
		var data bytes.Buffer
		dataReader := io.LimitReader(rand.Reader, int64(sinfo.MaxPieceSize.Uint64()))
		dataReader = io.TeeReader(dataReader, &data)
		dcid, deal, err := series.ImportAndStoreWithDuration(ctx, client, ask, dealExpiry, files.NewReaderFile(dataReader))
		require.NoError(t, err)

		t.Logf("Piece ID %s", dcid)
		t.Logf("Deal ID  %s", deal.ProposalCid)

		// Wait for the deal to be complete

		start := time.Now()
		_, err = series.WaitForDealState(ctx, client, deal, storagedeal.Complete)
		require.NoError(t, err)

		elapsed := time.Since(start)
		t.Logf("Storage deal took %s to move to `Complete` state", elapsed)

		//
		// Verify that deal shows up on both the miner and the client
		//

		_, err = series.FindDealByID(ctx, client, deal.ProposalCid)
		require.NoError(t, err, "Could not find deal on client")

		_, err = series.FindDealByID(ctx, miner, deal.ProposalCid)
		require.NoError(t, err, "Could not find deal on miner")

		//
		// Verify that the deal piece can be retrieved
		//

		// Retrieve the stored piece of data
		reader, err := client.RetrievalClientRetrievePiece(ctx, dcid, ask.Miner)
		require.NoError(t, err)

		// Verify that it's all the same
		retrievedData, err := ioutil.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, data.Bytes(), retrievedData)

		//
		// Verify that the deal voucher can be redeemed at the end of the deal
		//

		vouchers, err := client.ClientPayments(ctx, deal.ProposalCid)
		require.NoError(t, err)

		lastVoucher := vouchers[len(vouchers)-1]

		t.Logf("Vocuher Channel %s", lastVoucher.Channel.String())
		t.Logf("Voucher Amount  %s", lastVoucher.Amount.String())
		t.Logf("Voucher ValidAt %s", lastVoucher.ValidAt.String())

		err = series.WaitForBlockHeight(ctx, miner, &lastVoucher.ValidAt)
		require.NoError(t, err)

		var addr address.Address
		err = miner.ConfigGet(ctx, "wallet.defaultAddress", &addr)
		require.NoError(t, err)

		balanceBefore, err := miner.WalletBalance(ctx, addr)
		require.NoError(t, err)

		t.Logf("Balance Before %s", balanceBefore)

		mcid, err := miner.DealsRedeem(ctx, deal.ProposalCid, fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
		require.NoError(t, err)

		t.Logf("Redeem mcid    %s", mcid)

		result, err := miner.MessageWait(ctx, mcid)
		require.NoError(t, err)

		t.Logf("Redeem msg gas %s", result.Receipt.GasAttoFIL)

		balanceAfter, err := miner.WalletBalance(ctx, addr)
		require.NoError(t, err)

		t.Logf("Balance After  %s", balanceAfter)

		// We add the receipt back to the after balance to "undo" the gas costs, then subtract the before balance
		// what is left is the change as a result of redeeming the voucher
		assert.Equal(t, lastVoucher.Amount, balanceAfter.Add(result.Receipt.GasAttoFIL).Sub(balanceBefore))

		//
		// Verify that the miners power has increased
		//

		minfo, err := series.WaitForChainMessage(ctx, miner, func(ctx context.Context, node *fast.Filecoin, msg *types.SignedMessage) (bool, error) {
			if msg.Message.Method == minerActor.SubmitPoSt && msg.Message.To == ask.Miner {
				return true, nil
			}

			return false, nil
		})
		require.NoError(t, err)

		t.Logf("submitPoSt message        %s", minfo.MsgCid)
		t.Logf("submitPoSt found in block %s", minfo.BlockCid)

		resp, err := miner.MessageWait(ctx, minfo.MsgCid)
		require.NoError(t, err)
		assert.Equal(t, 0, int(resp.Receipt.ExitCode))

		mpower, err := miner.MinerPower(ctx, ask.Miner)
		require.NoError(t, err)

		// We should have a single sector of power
		assert.Equal(t, &mpower.Power, sinfo.Size)
	})
}
