package retrievalmarketconnector

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-fil-markets/piecestore"
	gfmtut "github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	tut "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/shared_testutils"

)

func TestNewRetrievalProviderNodeConnector(t *testing.T) {

}

func TestRetrievalProviderConnector_UnsealSector(t *testing.T) {

}

func TestRetrievalProviderConnector_SavePaymentVoucher(t *testing.T) {
	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	ps := &tut.DummyPieceStore{}
	bs := blockstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	rpc := NewRetrievalProviderNodeConnector(rmnet, ps, bs)

	ctx := context.Background()

	t.Run("saves payment voucher if it doesn't exist", func(t *testing.T) {
		tokenamt, err := rpc.SavePaymentVoucher()
	})

	t.Run("does not overwrite voucher if it exists", func(t *testing.T) {

	})
	t.Run("errors if cannot create a key for the voucher store", func(t *testing.T) {

	})

	t.Run("adds amount to voucher and returns total received")
}
