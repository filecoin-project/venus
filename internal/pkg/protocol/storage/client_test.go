package storage_test

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/convert"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

var testSignature = types.Signature("<test signature>")

func TestProposeDeal(t *testing.T) {
	t.Skip("Skip pending storage market integration")
	tf.UnitTest(t)

	ctx := context.Background()
	addressCreator := address.NewForTestGetter()

	var proposal *storagedeal.SignedProposal

	pieceSize := uint64(7)
	pieceReader := bytes.NewReader(make([]byte, pieceSize))
	testAPI := newTestClientAPI(t, pieceReader, pieceSize)
	client := NewClient()

	dataCid := types.CidFromString(t, "somecid")

	minerAddr := addressCreator()
	askID := uint64(67)
	duration := uint64(10000)
	dealResponse, err := client.ProposeDeal(ctx, minerAddr, dataCid, askID, duration, false)
	require.NoError(t, err)

	t.Run("and creates proposal from parameters", func(t *testing.T) {
		assert.Equal(t, dataCid, proposal.PieceRef)
		assert.Equal(t, duration, proposal.Duration)
		assert.Equal(t, minerAddr, proposal.MinerAddress)
		assert.Equal(t, testSignature, proposal.Signature)
	})

	t.Run("and creates proposal with file size", func(t *testing.T) {
		expectedFileSize, err := testAPI.DAGGetFileSize(ctx, dataCid)
		require.NoError(t, err)
		assert.Equal(t, types.NewBytesAmount(expectedFileSize), proposal.Size)
	})

	t.Run("and computes the correct total price", func(t *testing.T) {
		expectedAskPrice := types.NewAttoFILFromFIL(32) // from test plumbing
		expectedFileSize, err := testAPI.DAGGetFileSize(ctx, dataCid)
		require.NoError(t, err)

		expectedTotalPrice := expectedAskPrice.MulBigInt(big.NewInt(int64(expectedFileSize * duration)))
		assert.Equal(t, expectedTotalPrice, proposal.TotalPrice)
	})

	t.Run("and creates a new payment channel", func(t *testing.T) {
		// correct payment id and message cid in proposal implies a call to createChannel
		assert.Equal(t, testAPI.channelID, proposal.Payment.Channel)
		assert.Equal(t, &testAPI.msgCid, proposal.Payment.ChannelMsgCid)
	})

	t.Run("and creates payment info", func(t *testing.T) {
		assert.Equal(t, int(duration/VoucherInterval), len(proposal.Payment.Vouchers))

		lastValidAt := types.NewBlockHeight(0)
		for i, voucher := range proposal.Payment.Vouchers {
			assert.Equal(t, testAPI.channelID, &voucher.Channel)
			assert.True(t, voucher.ValidAt.GreaterThan(lastValidAt))
			assert.Equal(t, testAPI.target, voucher.Target)
			assert.Equal(t, testAPI.perPayment.MulBigInt(big.NewInt(int64(i+1))), voucher.Amount)
			assert.Equal(t, testAPI.payer, voucher.Payer)
			assert.Equal(t, minerAddr, voucher.Condition.To)
			lastValidAt = &voucher.ValidAt
		}
	})

	t.Run("and sends proposal and stores response", func(t *testing.T) {
		assert.NotNil(t, dealResponse)

		assert.Equal(t, storagedeal.Accepted, dealResponse.State)

		retrievedDeal, err := testAPI.DealGet(context.Background(), dealResponse.ProposalCid)
		require.NoError(t, err)

		assert.Equal(t, retrievedDeal.Response, dealResponse)
	})
}

func TestProposeZeroPriceDeal(t *testing.T) {
	t.Skip("Skip pending storage market integration")
	tf.UnitTest(t)

	ctx := context.Background()
	addressCreator := address.NewForTestGetter()

	// Create API and set miner's price to zero
	pieceSize := uint64(7)
	pieceReader := bytes.NewReader(make([]byte, pieceSize))
	testAPI := newTestClientAPI(t, pieceReader, pieceSize)
	testAPI.askPrice = types.ZeroAttoFIL

	client := NewClient()
	newTestClientNode(func(request interface{}) (interface{}, error) {
		p := request.(*storagedeal.SignedProposal)

		// assert that client does not send payment information in deal
		assert.Equal(t, types.ZeroAttoFIL, p.TotalPrice)

		assert.Equal(t, (*types.ChannelID)(nil), p.Payment.Channel)
		assert.Equal(t, address.Undef, p.Payment.PayChActor)
		assert.Equal(t, testAPI.payer, p.Payment.Payer)
		assert.Nil(t, p.Payment.Vouchers)

		pcid, err := convert.ToCid(p)
		require.NoError(t, err)

		resp := &storagedeal.SignedResponse{
			Response: storagedeal.Response{
				State:       storagedeal.Accepted,
				Message:     "OK",
				ProposalCid: pcid,
			},
		}
		require.NoError(t, resp.Sign(testAPI.signer, testAPI.worker))
		return resp, nil
	})

	_, err := client.ProposeDeal(ctx, addressCreator(), types.CidFromString(t, "somecid"), uint64(67), uint64(10000), false)
	require.NoError(t, err)

	// ensure client did not attempt to create a payment channel
	assert.False(t, testAPI.createdPayment)
}

func TestProposeDealFailsWhenADealAlreadyExists(t *testing.T) {
	t.Skip("Skip pending storage market integration")
	tf.UnitTest(t)

	ctx := context.Background()
	addressCreator := address.NewForTestGetter()

	client := NewClient()

	dataCid := types.CidFromString(t, "somecid")

	minerAddr := addressCreator()
	askID := uint64(67)
	duration := uint64(10000)
	_, err := client.ProposeDeal(ctx, minerAddr, dataCid, askID, duration, false)
	require.NoError(t, err)
	_, err = client.ProposeDeal(ctx, minerAddr, dataCid, askID, duration, false)
	assert.Error(t, err)
}

func TestProposeDealFailsWhenSignatureIsInvalid(t *testing.T) {
	t.Skip("Skip pending storage market integration")
	tf.UnitTest(t)

	ctx := context.Background()
	addressCreator := address.NewForTestGetter()

	client := NewClient()

	dataCid := types.CidFromString(t, "somecid")

	minerAddr := addressCreator()
	askID := uint64(67)
	duration := uint64(10000)
	_, err := client.ProposeDeal(ctx, minerAddr, dataCid, askID, duration, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature is invalid")
}

type clientTestAPI struct {
	askPrice       types.AttoFIL
	createdPayment bool
	blockHeight    uint64
	channelID      *types.ChannelID
	msgCid         cid.Cid
	payer          address.Address
	target         address.Address
	worker         address.Address
	perPayment     types.AttoFIL
	testing        *testing.T
	deals          map[cid.Cid]*storagedeal.Deal
	pieceReader    io.Reader
	pieceSize      uint64
	signer         types.Signer
}

func newTestClientAPI(t *testing.T, pieceReader io.Reader, pieceSize uint64) *clientTestAPI {
	cidGetter := types.NewCidForTestGetter()
	addressGetter := address.NewForTestGetter()
	mockSigner, ki := types.NewMockSignersAndKeyInfo(1)
	workerAddr, err := ki[0].Address()
	require.NoError(t, err, "Could not create worker address")

	return &clientTestAPI{
		askPrice:       types.NewAttoFILFromFIL(32),
		createdPayment: false,
		blockHeight:    773,
		msgCid:         cidGetter(),
		channelID:      types.NewChannelID(23),
		payer:          addressGetter(),
		target:         addressGetter(),
		worker:         workerAddr,
		perPayment:     types.NewAttoFILFromFIL(10),
		signer:         mockSigner,
		testing:        t,
		deals:          make(map[cid.Cid]*storagedeal.Deal),
		pieceReader:    pieceReader,
		pieceSize:      pieceSize,
	}
}

func (ctp *clientTestAPI) BlockTime() time.Duration {
	return 100 * time.Millisecond
}

func (ctp *clientTestAPI) ChainHeadKey() block.TipSetKey {
	return block.NewTipSetKey()
}

func (ctp *clientTestAPI) ChainTipSet(_ block.TipSetKey) (block.TipSet, error) {
	return block.NewTipSet(&block.Block{Height: types.Uint64(ctp.blockHeight)})
}

func (ctp *clientTestAPI) CreatePayments(ctx context.Context, config porcelain.CreatePaymentsParams) (*porcelain.CreatePaymentsReturn, error) {
	ctp.createdPayment = true
	resp := &porcelain.CreatePaymentsReturn{
		CreatePaymentsParams: config,
		Channel:              ctp.channelID,
		ChannelMsgCid:        ctp.msgCid,
		GasAttoFIL:           types.NewAttoFILFromFIL(100),
		Vouchers:             make([]*types.PaymentVoucher, 10),
	}

	conditionMethod := types.MethodID(28273)

	for i := 0; i < 10; i++ {
		resp.Vouchers[i] = &types.PaymentVoucher{
			Channel: *ctp.channelID,
			Payer:   ctp.payer,
			Target:  ctp.target,
			Amount:  ctp.perPayment.MulBigInt(big.NewInt(int64(i + 1))),
			ValidAt: *types.NewBlockHeight(ctp.blockHeight).Add(types.NewBlockHeight(uint64(i+1) * VoucherInterval)),
			Condition: &types.Predicate{
				To:     config.MinerAddress,
				Method: conditionMethod,
				Params: []interface{}{config.CommP[:]},
			},
		}
	}
	return resp, nil
}

func (ctp *clientTestAPI) DAGGetFileSize(context.Context, cid.Cid) (uint64, error) {
	return ctp.pieceSize, nil
}

func (ctp *clientTestAPI) DAGCat(context.Context, cid.Cid) (io.Reader, error) {
	return ctp.pieceReader, nil
}

func (ctp *clientTestAPI) MinerGetAsk(ctx context.Context, minerAddr address.Address, askID uint64) (miner.Ask, error) {
	return miner.Ask{
		Price:  ctp.askPrice,
		Expiry: types.NewBlockHeight(41),
		ID:     big.NewInt(4),
	}, nil
}

func (ctp *clientTestAPI) MinerGetOwnerAddress(ctx context.Context, minerAddr address.Address) (address.Address, error) {
	return address.TestAddress, nil
}

func (ctp *clientTestAPI) MinerGetWorkerAddress(_ context.Context, _ address.Address, _ block.TipSetKey) (address.Address, error) {
	return ctp.worker, nil
}

func (ctp *clientTestAPI) MinerGetSectorSize(ctx context.Context, minerAddr address.Address) (*types.BytesAmount, error) {
	return types.OneKiBSectorSize, nil
}

func (ctp *clientTestAPI) MinerGetPeerID(ctx context.Context, minerAddr address.Address) (peer.ID, error) {
	id, err := peer.IDB58Decode("QmWbMozPyW6Ecagtxq7SXBXXLY5BNdP1GwHB2WoZCKMvcb")
	require.NoError(ctp.testing, err, "Could not create peer id")

	return id, nil
}

func (ctp *clientTestAPI) PingMinerWithTimeout(ctx context.Context, p peer.ID, to time.Duration) error {
	return nil
}

func (ctp *clientTestAPI) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	return testSignature, nil
}

func (ctp *clientTestAPI) WalletDefaultAddress() (address.Address, error) {
	// always just default address
	return ctp.payer, nil
}

type testClientNode struct {
	responder func(request interface{}) (interface{}, error)
}

func newTestClientNode(responder func(request interface{}) (interface{}, error)) *testClientNode {
	return &testClientNode{
		responder: responder,
	}
}

// MakeTestProtocolRequest calls the responder set for the testClientNode to provide a test
// response for a protocol request.
// It ignores the host param required for the storage client interface.
func (tcn *testClientNode) MakeTestProtocolRequest(ctx context.Context, protocol protocol.ID, peer peer.ID, _ host.Host, request interface{}, response interface{}) error {
	dealResponse := response.(*storagedeal.SignedResponse)
	res, err := tcn.responder(request)
	if err != nil {
		return err
	}
	*dealResponse = *res.(*storagedeal.SignedResponse)
	return nil
}

func (ctp *clientTestAPI) DealsLs(_ context.Context) (<-chan *porcelain.StorageDealLsResult, error) {
	results := make(chan *porcelain.StorageDealLsResult)
	go func() {
		for _, deal := range ctp.deals {
			results <- &porcelain.StorageDealLsResult{
				Deal: *deal,
			}
		}
		close(results)
	}()
	return results, nil
}

func (ctp *clientTestAPI) DealGet(_ context.Context, dealCid cid.Cid) (*storagedeal.Deal, error) {
	deal, ok := ctp.deals[dealCid]
	if ok {
		return deal, nil
	}
	return nil, porcelain.ErrDealNotFound
}

func (ctp *clientTestAPI) DealPut(storageDeal *storagedeal.Deal) error {
	ctp.deals[storageDeal.Response.ProposalCid] = storageDeal
	return nil
}

func (ctp *clientTestAPI) MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, _ block.TipSetKey, params ...interface{}) ([][]byte, error) {
	return [][]byte{{byte(types.TestProofsMode)}}, nil
}
