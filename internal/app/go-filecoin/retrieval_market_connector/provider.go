package retrievalmarketconnector

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
)

type RetrievalProviderNodeConnector struct{}

func NewRetrievalProviderNodeConnector() *RetrievalProviderNodeConnector {
	return &RetrievalProviderNodeConnector{}
}

func (r RetrievalProviderNodeConnector) UnsealSector(ctx context.Context, sectorId uint64, offset uint64, length uint64) (io.ReadCloser, error) {
	panic("TODO: go-fil-markets integration")
}

// SavePaymentVoucher saves a payment voucher
func (r RetrievalProviderNodeConnector) SavePaymentVoucher(ctx context.Context, paymentChannel address.Address, voucher *paych.SignedVoucher, proof []byte, expectedAmount abi.TokenAmount) (abi.TokenAmount, error) {
	panic("TODO: go-fil-markets integration")
}
