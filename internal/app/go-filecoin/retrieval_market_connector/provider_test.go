package retrieval_market_connector_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	"github.com/filecoin-project/go-fil-markets/shared/types"
	gfmtut "github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-address"

	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	tut "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/shared_testutils"
)

func TestNewRetrievalProviderNodeConnector(t *testing.T) {

}

func TestRetrievalProviderConnector_UnsealSector(t *testing.T) {

}

func TestRetrievalProviderConnector_SavePaymentVoucher(t *testing.T) {
	rmnet := gfmtut.NewTestRetrievalMarketNetwork(gfmtut.TestNetworkParams{})
	ps := tut.RequireMakeTestPieceStore(t)
	bs := blockstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	pchan, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	ctx := context.Background()
	voucher := &types.SignedVoucher{
		Lane:           rand.Uint64(),
		Nonce:          rand.Uint64(),
		Amount:         tokenamount.FromInt(rand.Uint64()),
		MinCloseHeight: rand.Uint64(),
	}
	dealAmount := tokenamount.FromInt(rand.Uint64())
	proof := []byte("proof")

	t.Run("saves payment voucher and returns voucher amount if new", func(t *testing.T) {
		rpc := NewRetrievalProviderNodeConnector(rmnet, ps, &bs)
		tokenamt, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, dealAmount)
		assert.NoError(t, err)
		assert.True(t, voucher.Amount.Equals(tokenamt))
	})

	t.Run("errors and does not overwrite voucher if already saved, regardless of other params", func(t *testing.T) {
		rpc := NewRetrievalProviderNodeConnector(rmnet, ps, &bs)
		_, err := rpc.SavePaymentVoucher(ctx, pchan, voucher, proof, dealAmount)
		require.NoError(t, err)
		proof2 := []byte("newproof")
		dealAmount2 := tokenamount.FromInt(rand.Uint64())
		pchan2, err := address.NewIDAddress(rand.Uint64())
		tokenamt2, err := rpc.SavePaymentVoucher(ctx, pchan2, voucher, proof2, dealAmount2)
		assert.EqualError(t, err, "voucher exists")
		assert.Equal(t, tokenamount.TokenAmount{nil}, tokenamt2)
	})
	t.Run("errors if cannot create a key for the voucher store", func(t *testing.T) {
		rpc := NewRetrievalProviderNodeConnector(rmnet, ps, &bs)
		badVoucher := &types.SignedVoucher{
			Merges: make([]types.Merge, cbg.MaxLength+1),
		}

		_, err := rpc.SavePaymentVoucher(ctx, pchan, badVoucher, proof, dealAmount)
		assert.EqualError(t, err, "Slice value in field t.Merges was too long")
	})
}
