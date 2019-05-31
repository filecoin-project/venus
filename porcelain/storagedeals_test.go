package porcelain_test

import (
	"context"
	"testing"

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
