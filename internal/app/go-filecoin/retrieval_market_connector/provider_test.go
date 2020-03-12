package retrievalmarketconnector_test

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
	"testing"

	"github.com/filecoin-project/go-address"
	gfmtut "github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	specst "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pch "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestNewRetrievalProviderNodeConnector(t *testing.T) {
	tf.UnitTest(t)
	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	pm := piecemanager.NewStorageMinerBackEnd(nil, nil)
	bs := blockstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))

	pchMgr := makePaychMgr(context.Background(), t,
		specst.NewIDAddr(t, 99),
		specst.NewIDAddr(t, 100),
		specst.NewActorAddr(t, "foobar"))
	rpc := NewRetrievalProviderConnector(rmnet, pm, bs, pchMgr)
	assert.NotZero(t, rpc)
}

func TestRetrievalProviderConnector_UnsealSector(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	sectorID := rand.Uint64()
	fixtureFile := "../../../../fixtures/constants.go"

	intSz := reflect.TypeOf(0).Size()*8 - 1
	maxOffset := uint64(1 << intSz)

	testCases := []struct {
		name                        string
		offset, length, expectedLen uint64
		unsealErr                   error
		expectedErr                 string
	}{
		{name: "happy path", offset: 10, length: 50, expectedLen: 50, expectedErr: ""},
		{name: "happy even if length more than file length", offset: 10, length: 9999, expectedLen: 4086, expectedErr: ""},
		{name: "returns error if Unseal errors", unsealErr: errors.New("boom"), expectedErr: "boom"},
		{name: "returns EOF if offset more than file length", offset: 9999, expectedErr: "EOF"},
		{name: "returns error if offset > int64", offset: maxOffset, expectedErr: "offset overflows int"},
		{name: "returns error if length > int64", length: 1 << 63, expectedErr: "length overflows int64"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rmp, rpc := unsealTestSetup(ctx, t)
			rmp.ExpectedSectorIDs[sectorID] = fixtureFile

			if tc.expectedErr != "" {
				rmp.UnsealErr = tc.unsealErr
				_, err := rpc.UnsealSector(ctx, sectorID, tc.offset, tc.length)
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				res, err := rpc.UnsealSector(ctx, sectorID, tc.offset, tc.length)
				require.NoError(t, err)
				readBytes := make([]byte, tc.length+1)
				readlen, err := res.Read(readBytes)
				require.NoError(t, err)
				assert.Equal(t, int(tc.expectedLen), readlen)

				// check that it read something & the offset worked
				assert.Equal(t, "xtures", string(readBytes[0:6]))
				rmp.Verify()
			}
		})
	}
}

func unsealTestSetup(ctx context.Context, t *testing.T) (*RetrievalMarketClientFakeAPI, *RetrievalProviderConnector) {
	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	bs := blockstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	rmp := NewRetrievalMarketClientFakeAPI(t, abi.NewTokenAmount(0))
	pchMgr := makePaychMgr(ctx, t,
		specst.NewIDAddr(t, 99),
		specst.NewIDAddr(t, 100),
		specst.NewActorAddr(t, "foobar"))
	rpc := NewRetrievalProviderConnector(rmnet, rmp, bs, pchMgr)
	return rmp, rpc
}

func TestRetrievalProviderConnector_SavePaymentVoucher(t *testing.T) {
	ctx := context.Background()

	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	pm := piecemanager.NewStorageMinerBackEnd(nil, nil)

	bs := blockstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	pchan := specst.NewIDAddr(t, 100)
	clientAddr := specst.NewIDAddr(t, 101)
	minerAddr := specst.NewIDAddr(t, 102)
	root := gfmtut.GenerateCids(1)[0]

	viewer, pchMgr := makeViewerAndManager(ctx, t, clientAddr, minerAddr, pchan, root)
	viewer.Views[root].AddActorWithState(pchan, clientAddr, minerAddr, address.Undef)

	voucher := &paych.SignedVoucher{
		Lane:            rand.Uint64(),
		Nonce:           rand.Uint64(),
		Amount:          big.NewInt(rand.Int63()),
		MinSettleHeight: abi.ChainEpoch(99),
	}
	proof := []byte("proof")

	t.Run("saves payment voucher and returns voucher amount if new", func(t *testing.T) {
		rmp := NewRetrievalMarketClientFakeAPI(t, abi.NewTokenAmount(0))
		// simulate creating payment channel
		rmp.ExpectedVouchers[pchan] = &pch.VoucherInfo{Voucher: voucher, Proof: proof}

		rpc := NewRetrievalProviderConnector(rmnet, pm, bs, pchMgr)

		tokenamt, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, voucher.Amount)
		assert.NoError(t, err)
		assert.True(t, voucher.Amount.Equals(tokenamt))

		chinfo, err := pchMgr.GetPaymentChannelInfo(pchan)
		require.NoError(t, err)
		assert.True(t, chinfo.HasVoucher(voucher))
		rmp.Verify()
	})

	t.Run("errors if manager fails to save voucher", func(t *testing.T) {
		viewer.Views[root].PaychActorPartiesErr = errors.New("boom")

		rmp := NewRetrievalMarketClientFakeAPI(t, abi.NewTokenAmount(0))
		rmp.ExpectedVouchers[pchan] = &pch.VoucherInfo{Voucher: voucher, Proof: proof}
		rpc := NewRetrievalProviderConnector(rmnet, pm, bs, pchMgr)
		_, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, voucher.Amount)
		assert.EqualError(t, err, "boom")
		chinfo, err := pchMgr.GetPaymentChannelInfo(pchan)
		require.NoError(t, err)
		assert.False(t, chinfo.HasVoucher(voucher))
	})
}

func makeViewerAndManager(ctx context.Context, t *testing.T, client, miner, paych address.Address, root state.Root) (*pch.FakeStateViewer, *pch.Manager) {
	ds := dss.MutexWrap(datastore.NewMapDatastore())
	testAPI := pch.NewFakePaymentChannelAPI(ctx, t)
	viewer := makeStateViewer(t, root, nil)
	cr := pch.NewFakeChainReader(block.NewTipSetKey(root))
	pchMgr := pch.NewManager(context.Background(), ds, testAPI, testAPI, viewer, cr)
	blockHeight := uint64(1234)

	testAPI.StubCreatePaychActorMessage(t, client, miner, paych, exitcode.Ok, blockHeight)
	return viewer, pchMgr
}
