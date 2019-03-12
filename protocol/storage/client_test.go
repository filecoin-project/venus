package storage

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/util/convert"
)

var testSignature = types.Signature("<test signature>")

func TestProposeDeal(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

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

	client, err := NewClient(testNode, testAPI)
	require.NoError(err)

	dataCid := types.SomeCid()
	minerAddr := addressCreator()
	ctx := context.Background()
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

func (ctp *clientTestAPI) ChainBlockHeight(ctx context.Context) (*types.BlockHeight, error) {
	return ctp.blockHeight, nil
}

func (ctp *clientTestAPI) CreatePayments(ctx context.Context, config porcelain.CreatePaymentsParams) (*porcelain.CreatePaymentsReturn, error) {
	resp := &porcelain.CreatePaymentsReturn{
		CreatePaymentsParams: config,
		Channel:              ctp.channelID,
		ChannelMsgCid:        ctp.msgCid,
		GasAttoFIL:           types.NewAttoFILFromFIL(100),
		Vouchers:             make([]*paymentbroker.PaymentVoucher, 10),
	}

	for i := 0; i < 10; i++ {
		resp.Vouchers[i] = &paymentbroker.PaymentVoucher{
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
	return 1000000000, nil
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

func (ctp *clientTestAPI) NetworkPing(ctx context.Context, p peer.ID) (<-chan time.Duration, error) {
	out := make(chan time.Duration, 1)
	out <- 0
	close(out)
	return out, nil
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

func (tcn *testClientNode) MakeProtocolRequest(ctx context.Context, protocol protocol.ID, peer peer.ID, request interface{}, response interface{}) error {
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
