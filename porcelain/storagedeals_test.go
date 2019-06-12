package porcelain_test

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

type testDealLsPlumbing struct {
	deals        []*storagedeal.Deal
	minerAddress address.Address
}

func (tdlp *testDealLsPlumbing) DealsLs(_ context.Context) (<-chan *porcelain.StorageDealLsResult, error) {
	dealCh := make(chan *porcelain.StorageDealLsResult)
	go func() {
		for _, deal := range tdlp.deals {
			dealCh <- &porcelain.StorageDealLsResult{
				Deal: *deal,
			}
		}
		close(dealCh)
	}()
	return dealCh, nil
}

func (tdlp *testDealLsPlumbing) ConfigGet(path string) (interface{}, error) {
	return tdlp.minerAddress, nil
}

func TestDealGet(t *testing.T) {
	tf.UnitTest(t)

	cidGetter := types.NewCidForTestGetter()
	dealCid := cidGetter()
	expectedDeal := &storagedeal.Deal{
		Response: &storagedeal.Response{
			ProposalCid: dealCid,
		},
	}

	plumbing := &testDealLsPlumbing{
		deals: []*storagedeal.Deal{expectedDeal},
	}

	resultDeal, err := porcelain.DealGet(context.Background(), plumbing, dealCid)
	assert.NoError(t, err)
	assert.NotNil(t, resultDeal)
	assert.Equal(t, expectedDeal, resultDeal)
}

func TestDealGetNotFound(t *testing.T) {
	tf.UnitTest(t)

	cidGetter := types.NewCidForTestGetter()
	dealCid := cidGetter()
	badCid := cidGetter()
	expectedDeal := &storagedeal.Deal{
		Response: &storagedeal.Response{
			ProposalCid: dealCid,
		},
	}

	plumbing := &testDealLsPlumbing{
		deals: []*storagedeal.Deal{expectedDeal},
	}

	resultDeal, err := porcelain.DealGet(context.Background(), plumbing, badCid)
	assert.Error(t, err, porcelain.ErrDealNotFound)
	assert.Nil(t, resultDeal)
}

func TestDealClientLs(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()
	ownAddress := addrGetter()
	clientDeal := &storagedeal.Deal{
		Miner: addrGetter(),
	}
	minerDeal := &storagedeal.Deal{
		Miner: ownAddress,
	}

	plumbing := &testDealLsPlumbing{
		deals: []*storagedeal.Deal{
			clientDeal,
			minerDeal,
		},
		minerAddress: ownAddress,
	}

	var results []*storagedeal.Deal
	resultsCh, err := porcelain.DealClientLs(context.Background(), plumbing)
	require.NoError(t, err)
	for result := range resultsCh {
		require.NoError(t, result.Err)
		results = append(results, &result.Deal)
	}
	assert.Contains(t, results, clientDeal)
	assert.NotContains(t, results, minerDeal)
}

func TestDealMinerLs(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()
	ownAddress := addrGetter()
	clientDeal := &storagedeal.Deal{
		Miner: addrGetter(),
	}
	minerDeal := &storagedeal.Deal{
		Miner: ownAddress,
	}

	plumbing := &testDealLsPlumbing{
		deals: []*storagedeal.Deal{
			clientDeal,
			minerDeal,
		},
		minerAddress: ownAddress,
	}

	var results []*storagedeal.Deal
	resultsCh, err := porcelain.DealMinerLs(context.Background(), plumbing)
	require.NoError(t, err)
	for result := range resultsCh {
		require.NoError(t, result.Err)
		results = append(results, &result.Deal)
	}
	assert.NotContains(t, results, clientDeal)
	assert.Contains(t, results, minerDeal)
}

type testRedeemPlumbing struct {
	t *testing.T

	blockHeight     *types.BlockHeight
	dealCid         cid.Cid
	expectedVoucher *types.PaymentVoucher
	fromAddr        address.Address
	gasPrice        types.GasUnits
	messageCid      cid.Cid
	vouchers        []*types.PaymentVoucher
}

func (trp *testRedeemPlumbing) ChainBlockHeight() (*types.BlockHeight, error) {
	return trp.blockHeight, nil
}

func (trp *testRedeemPlumbing) DealGet(_ context.Context, c cid.Cid) (*storagedeal.Deal, error) {
	require.Equal(trp.t, trp.dealCid, c)

	deal := &storagedeal.Deal{
		Proposal: &storagedeal.Proposal{
			Payment: storagedeal.PaymentInfo{
				Vouchers: trp.vouchers,
			},
		},
	}

	return deal, nil
}

func (trp *testRedeemPlumbing) MessagePreview(_ context.Context, fromAddr address.Address, actorAddr address.Address, method string, params ...interface{}) (types.GasUnits, error) {
	require.Equal(trp.t, trp.fromAddr, fromAddr)
	require.Equal(trp.t, address.PaymentBrokerAddress, actorAddr)
	require.Equal(trp.t, "redeem", method)
	require.Equal(trp.t, []interface{}{
		trp.expectedVoucher.Payer,
		&trp.expectedVoucher.Channel,
		trp.expectedVoucher.Amount,
		&trp.expectedVoucher.ValidAt,
		trp.expectedVoucher.Condition,
		[]byte(trp.expectedVoucher.Signature),
		[]interface{}{},
	}, params)
	return trp.gasPrice, nil
}

func (trp *testRedeemPlumbing) MessageSendWithDefaultAddress(_ context.Context, fromAddr address.Address, actorAddr address.Address, _ types.AttoFIL, _ types.AttoFIL, _ types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	require.Equal(trp.t, trp.fromAddr, fromAddr)
	require.Equal(trp.t, address.PaymentBrokerAddress, actorAddr)
	require.Equal(trp.t, "redeem", method)
	require.Equal(trp.t, []interface{}{
		trp.expectedVoucher.Payer,
		&trp.expectedVoucher.Channel,
		trp.expectedVoucher.Amount,
		&trp.expectedVoucher.ValidAt,
		trp.expectedVoucher.Condition,
		[]byte(trp.expectedVoucher.Signature),
		[]interface{}{},
	}, params)
	return trp.messageCid, nil
}

func TestDealRedeem(t *testing.T) {
	tf.UnitTest(t)

	addressGetter := address.NewForTestGetter()
	cidGetter := types.NewCidForTestGetter()
	dealCid := cidGetter()
	messageCid := cidGetter()
	fromAddr := addressGetter()
	payerAddr := addressGetter()
	channelID := *types.NewChannelID(0)
	tooSmallVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: channelID,
		Amount:  types.NewAttoFILFromFIL(1),
		ValidAt: *types.NewBlockHeight(10),
	}
	expectedVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: channelID,
		Amount:  types.NewAttoFILFromFIL(2),
		ValidAt: *types.NewBlockHeight(20),
	}
	notYetValidVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: channelID,
		Amount:  types.NewAttoFILFromFIL(3),
		ValidAt: *types.NewBlockHeight(30),
	}
	vouchers := []*types.PaymentVoucher{
		tooSmallVoucher,
		expectedVoucher,
		notYetValidVoucher,
	}
	plumbing := &testRedeemPlumbing{
		t:               t,
		blockHeight:     types.NewBlockHeight(25),
		dealCid:         dealCid,
		expectedVoucher: expectedVoucher,
		fromAddr:        fromAddr,
		messageCid:      messageCid,
		vouchers:        vouchers,
	}

	resultCid, err := porcelain.DealRedeem(context.Background(), plumbing, fromAddr, dealCid, types.NewAttoFILFromFIL(0), types.NewGasUnits(0))
	require.NoError(t, err)

	assert.Equal(t, messageCid, resultCid)
}

func TestDealRedeemPreview(t *testing.T) {
	tf.UnitTest(t)

	addressGetter := address.NewForTestGetter()
	cidGetter := types.NewCidForTestGetter()
	dealCid := cidGetter()
	fromAddr := addressGetter()
	payerAddr := addressGetter()
	channelID := *types.NewChannelID(0)
	tooSmallVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: channelID,
		Amount:  types.NewAttoFILFromFIL(1),
		ValidAt: *types.NewBlockHeight(10),
	}
	expectedVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: channelID,
		Amount:  types.NewAttoFILFromFIL(2),
		ValidAt: *types.NewBlockHeight(20),
	}
	notYetValidVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: channelID,
		Amount:  types.NewAttoFILFromFIL(3),
		ValidAt: *types.NewBlockHeight(30),
	}
	vouchers := []*types.PaymentVoucher{
		tooSmallVoucher,
		expectedVoucher,
		notYetValidVoucher,
	}
	plumbing := &testRedeemPlumbing{
		t:               t,
		blockHeight:     types.NewBlockHeight(25),
		dealCid:         dealCid,
		expectedVoucher: expectedVoucher,
		fromAddr:        fromAddr,
		gasPrice:        types.NewGasUnits(42),
		vouchers:        vouchers,
	}

	resultGasPrice, err := porcelain.DealRedeemPreview(context.Background(), plumbing, fromAddr, dealCid)
	require.NoError(t, err)

	assert.Equal(t, types.NewGasUnits(42), resultGasPrice)
}
