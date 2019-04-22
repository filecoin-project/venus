package storage_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-host"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/libp2p/go-libp2p-protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	. "github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/util/convert"
)

var testSignature = types.Signature("<test signature>")

func TestProposeDeal(t *testing.T) {
	tf.UnitTest(t)

	require := require.New(t)
	assert := assert.New(t)
	ctx := context.Background()
	addressCreator := address.NewForTestGetter()

	var proposal *storagedeal.SignedDealProposal

	testNode := newTestClientNode(func(request interface{}) (interface{}, error) {
		p, ok := request.(*storagedeal.SignedDealProposal)
		require.True(ok)
		proposal = p

		pcid, err := convert.ToCid(p.Proposal)
		require.NoError(err)
		return &storagedeal.Response{
			State:       storagedeal.Accepted,
			Message:     "OK",
			ProposalCid: pcid,
		}, nil
	})

	testAPI := newTestClientAPI(require)
	client := NewClient(testNode.GetBlockTime(), th.NewFakeHost(), testAPI)
	client.ProtocolRequestFunc = testNode.MakeTestProtocolRequest

	dataCid := types.SomeCid()
	minerAddr := addressCreator()
	askID := uint64(67)
	duration := uint64(10000)
	dealResponse, err := client.ProposeDeal(ctx, minerAddr, dataCid, askID, duration, false)
	require.NoError(err)

	t.Run("and creates proposal from parameters", func(t *testing.T) {
		assert.Equal(dataCid, proposal.PieceRef)
		assert.Equal(duration, proposal.Duration)
		assert.Equal(minerAddr, proposal.MinerAddress)
		assert.Equal(testSignature, proposal.Signature)
	})

	t.Run("and creates proposal with file size", func(t *testing.T) {
		expectedFileSize, err := testAPI.DAGGetFileSize(ctx, dataCid)
		require.NoError(err)
		assert.Equal(types.NewBytesAmount(expectedFileSize), proposal.Size)
	})

	t.Run("and computes the correct total price", func(t *testing.T) {
		expectedAskPrice := types.NewAttoFILFromFIL(32) // from test plumbing
		expectedFileSize, err := testAPI.DAGGetFileSize(ctx, dataCid)
		require.NoError(err)

		expectedTotalPrice := expectedAskPrice.MulBigInt(big.NewInt(int64(expectedFileSize * duration)))
		assert.Equal(expectedTotalPrice, proposal.TotalPrice)
	})

	t.Run("and creates a new payment channel", func(t *testing.T) {
		// correct payment id and message cid in proposal implies a call to createChannel
		assert.Equal(testAPI.channelID, proposal.Payment.Channel)
		assert.Equal(&testAPI.msgCid, proposal.Payment.ChannelMsgCid)
	})

	t.Run("and creates payment info", func(t *testing.T) {
		assert.Equal(int(duration/VoucherInterval), len(proposal.Payment.Vouchers))

		lastValidAt := types.NewBlockHeight(0)
		for i, voucher := range proposal.Payment.Vouchers {
			assert.Equal(testAPI.channelID, &voucher.Channel)
			assert.True(voucher.ValidAt.GreaterThan(lastValidAt))
			assert.Equal(testAPI.target, voucher.Target)
			assert.Equal(testAPI.perPayment.MulBigInt(big.NewInt(int64(i+1))), &voucher.Amount)
			assert.Equal(testAPI.payer, voucher.Payer)
			lastValidAt = &voucher.ValidAt
		}
	})

	t.Run("and sends proposal and stores response", func(t *testing.T) {
		assert.NotNil(dealResponse)

		assert.Equal(storagedeal.Accepted, dealResponse.State)

		retrievedDeal := testAPI.DealGet(dealResponse.ProposalCid)

		assert.Equal(retrievedDeal.Response, dealResponse)
	})
}

type clientTestAPI struct {
	blockHeight *types.BlockHeight
	channelID   *types.ChannelID
	msgCid      cid.Cid
	payer       address.Address
	target      address.Address
	perPayment  *types.AttoFIL
	require     *require.Assertions
	deals       map[cid.Cid]*storagedeal.Deal
}

func newTestClientAPI(require *require.Assertions) *clientTestAPI {
	cidGetter := types.NewCidForTestGetter()
	addressGetter := address.NewForTestGetter()

	return &clientTestAPI{
		blockHeight: types.NewBlockHeight(773),
		msgCid:      cidGetter(),
		channelID:   types.NewChannelID(23),
		payer:       addressGetter(),
		target:      addressGetter(),
		perPayment:  types.NewAttoFILFromFIL(10),
		require:     require,
		deals:       make(map[cid.Cid]*storagedeal.Deal),
	}
}

func (ctp *clientTestAPI) ChainBlockHeight() (*types.BlockHeight, error) {
	return ctp.blockHeight, nil
}

func (ctp *clientTestAPI) CreatePayments(ctx context.Context, config porcelain.CreatePaymentsParams) (*porcelain.CreatePaymentsReturn, error) {
	resp := &porcelain.CreatePaymentsReturn{
		CreatePaymentsParams: config,
		Channel:              ctp.channelID,
		ChannelMsgCid:        ctp.msgCid,
		GasAttoFIL:           types.NewAttoFILFromFIL(100),
		Vouchers:             make([]*types.PaymentVoucher, 10),
	}

	for i := 0; i < 10; i++ {
		resp.Vouchers[i] = &types.PaymentVoucher{
			Channel: *ctp.channelID,
			Payer:   ctp.payer,
			Target:  ctp.target,
			Amount:  *ctp.perPayment.MulBigInt(big.NewInt(int64(i + 1))),
			ValidAt: *ctp.blockHeight.Add(types.NewBlockHeight(uint64(i+1) * VoucherInterval)),
		}
	}
	return resp, nil
}

func (ctp *clientTestAPI) DAGGetFileSize(context.Context, cid.Cid) (uint64, error) {
	return 1016, nil
}

func (ctp *clientTestAPI) MinerGetAsk(ctx context.Context, minerAddr address.Address, askID uint64) (miner.Ask, error) {
	return miner.Ask{
		Price:  types.NewAttoFILFromFIL(32),
		Expiry: types.NewBlockHeight(41),
		ID:     big.NewInt(4),
	}, nil
}

func (ctp *clientTestAPI) MinerGetOwnerAddress(ctx context.Context, minerAddr address.Address) (address.Address, error) {
	return address.TestAddress, nil
}

func (ctp *clientTestAPI) MinerGetPeerID(ctx context.Context, minerAddr address.Address) (peer.ID, error) {
	id, err := peer.IDB58Decode("QmWbMozPyW6Ecagtxq7SXBXXLY5BNdP1GwHB2WoZCKMvcb")
	ctp.require.NoError(err, "Could not create peer id")

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

func (tcn *testClientNode) GetBlockTime() time.Duration {
	return 100 * time.Millisecond
}

// MakeTestProtocolRequest calls the responder set for the testClientNode to provide a test
// response for a protocol request.
// It ignores the host param required for the storage client interface.
func (tcn *testClientNode) MakeTestProtocolRequest(ctx context.Context, protocol protocol.ID, peer peer.ID, _ host.Host, request interface{}, response interface{}) error {
	dealResponse := response.(*storagedeal.Response)
	res, err := tcn.responder(request)
	if err != nil {
		return err
	}
	*dealResponse = *res.(*storagedeal.Response)
	return nil
}

func (ctp *clientTestAPI) DealsLs() ([]*storagedeal.Deal, error) {
	var results []*storagedeal.Deal

	for _, storageDeal := range ctp.deals {
		results = append(results, storageDeal)
	}
	return results, nil
}

func (ctp *clientTestAPI) DealGet(dealCid cid.Cid) *storagedeal.Deal {
	return ctp.deals[dealCid]
}

func (ctp *clientTestAPI) DealPut(storageDeal *storagedeal.Deal) error {
	ctp.deals[storageDeal.Response.ProposalCid] = storageDeal
	return nil
}

func (ctp *clientTestAPI) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	return [][]byte{{byte(types.TestProofsMode)}}, nil
}
