package porcelain_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
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
		Response: &storagedeal.SignedResponse{
			Response: storagedeal.Response{
				ProposalCid: dealCid,
			},
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
		Response: &storagedeal.SignedResponse{
			Response: storagedeal.Response{
				ProposalCid: dealCid,
			},
		},
	}

	plumbing := &testDealLsPlumbing{
		deals: []*storagedeal.Deal{expectedDeal},
	}

	resultDeal, err := porcelain.DealGet(context.Background(), plumbing, badCid)
	assert.Error(t, err, porcelain.ErrDealNotFound)
	assert.Nil(t, resultDeal)
}

type testRedeemPlumbing struct {
	t *testing.T

	blockHeight uint64
	dealCid     cid.Cid
	gasPrice    types.GasUnits
	messageCid  cid.Cid
	vouchers    []*types.PaymentVoucher

	ResultingFromAddr       address.Address
	ResultingActorAddr      address.Address
	ResultingMethod         types.MethodID
	ResultingVoucherPayer   address.Address
	ResultingVoucherChannel *types.ChannelID
	ResultingVoucherAmount  types.AttoFIL
	ResultingVoucherValidAt *types.BlockHeight
}

func (trp *testRedeemPlumbing) ChainHeadKey() block.TipSetKey {
	return block.NewTipSetKey()
}

func (trp *testRedeemPlumbing) ChainTipSet(_ block.TipSetKey) (block.TipSet, error) {
	return block.NewTipSet(&block.Block{Height: types.Uint64(trp.blockHeight)})
}

func (trp *testRedeemPlumbing) DealGet(_ context.Context, c cid.Cid) (*storagedeal.Deal, error) {
	require.Equal(trp.t, trp.dealCid, c)

	deal := &storagedeal.Deal{
		Proposal: &storagedeal.SignedProposal{
			Proposal: storagedeal.Proposal{
				Payment: storagedeal.PaymentInfo{
					Vouchers: trp.vouchers,
				},
			},
		},
	}

	return deal, nil
}

func (trp *testRedeemPlumbing) MessagePreview(_ context.Context, fromAddr address.Address, actorAddr address.Address, method types.MethodID, params ...interface{}) (types.GasUnits, error) {
	trp.ResultingFromAddr = fromAddr
	trp.ResultingActorAddr = actorAddr
	trp.ResultingMethod = method
	trp.ResultingVoucherPayer = params[0].(address.Address)
	trp.ResultingVoucherChannel = params[1].(*types.ChannelID)
	trp.ResultingVoucherAmount = params[2].(types.AttoFIL)
	trp.ResultingVoucherValidAt = params[3].(*types.BlockHeight)
	return trp.gasPrice, nil
}

func (trp *testRedeemPlumbing) MessageSend(_ context.Context, fromAddr address.Address, actorAddr address.Address, _ types.AttoFIL, _ types.AttoFIL, _ types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error) {
	trp.ResultingFromAddr = fromAddr
	trp.ResultingActorAddr = actorAddr
	trp.ResultingMethod = method
	trp.ResultingVoucherPayer = params[0].(address.Address)
	trp.ResultingVoucherChannel = params[1].(*types.ChannelID)
	trp.ResultingVoucherAmount = params[2].(types.AttoFIL)
	trp.ResultingVoucherValidAt = params[3].(*types.BlockHeight)
	return trp.messageCid, nil, nil
}
