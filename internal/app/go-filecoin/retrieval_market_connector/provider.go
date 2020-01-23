package retrievalmarketconnector

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	"github.com/filecoin-project/go-fil-markets/shared/types"
)

type RetrievalProviderNodeConnector struct{}

func NewRetrievalProviderNodeConnector() *RetrievalProviderNodeConnector {
	return &RetrievalProviderNodeConnector{}
}

func (r RetrievalProviderNodeConnector) UnsealSector(ctx context.Context, sectorId uint64, offset uint64, length uint64) (io.ReadCloser, error) {
	panic("TODO: go-fil-markets integration")
}

func (r RetrievalProviderNodeConnector) SavePaymentVoucher(ctx context.Context, paymentChannel address.Address, voucher *types.SignedVoucher, proof []byte, expectedAmount tokenamount.TokenAmount) (tokenamount.TokenAmount, error) {
	panic("TODO: go-fil-markets integration")
}
