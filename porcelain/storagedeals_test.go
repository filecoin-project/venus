package porcelain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

type testDealLsPlumbing struct {
	deals        []*storagedeal.Deal
	minerAddress address.Address
}

func (tdlp *testDealLsPlumbing) DealsLs() ([]*storagedeal.Deal, error) {
	return tdlp.deals, nil
}

func (tdlp *testDealLsPlumbing) ConfigGet(path string) (interface{}, error) {
	return tdlp.minerAddress, nil
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

	results, err := porcelain.DealClientLs(plumbing)
	require.NoError(t, err)
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

	results, err := porcelain.DealMinerLs(plumbing)
	require.NoError(t, err)
	assert.NotContains(t, results, clientDeal)
	assert.Contains(t, results, minerDeal)
}
