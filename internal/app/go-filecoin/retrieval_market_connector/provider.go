package retrievalmarketconnector

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
)

// RetrievalProviderNodeConnector adapts the node to provide an interface to the retrieval provider
type RetrievalProviderNodeConnector struct{}

// NewRetrievalProviderNodeConnector creates a new connector
func NewRetrievalProviderNodeConnector() *RetrievalProviderNodeConnector {
	return &RetrievalProviderNodeConnector{}
}

// UnsealSector unseals a sector so that its pieces may be retrieved
func (r RetrievalProviderNodeConnector) UnsealSector(ctx context.Context, sectorID uint64, offset uint64, length uint64) (io.ReadCloser, error) {
	panic("TODO: go-fil-markets integration")
}

// SavePaymentVoucher saves a payment voucher
func (r RetrievalProviderNodeConnector) SavePaymentVoucher(ctx context.Context, paymentChannel address.Address, voucher *paych.SignedVoucher, proof []byte, expectedAmount abi.TokenAmount) (abi.TokenAmount, error) {
	panic("TODO: go-fil-markets integration")
}
